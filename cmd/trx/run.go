package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trx/internal/config"
	"trx/internal/git"
	"trx/internal/hooks"
	"trx/internal/lock"
	"trx/internal/quorum"
	"trx/internal/storage"
	"trx/internal/tasks"
	"trx/internal/templates"
)

type Executor interface {
	RunTasks(tasks []tasks.Task) error
}

func run(opts runOptions) error {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
	log.Println("Running trx")
	log.Printf("Start at %s\n", time.Now().Format("2006-01-02 15:04:05"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		log.Printf("Received signal: %s", sig)
		cancel()
	}()

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		return err
	}

	locker := lock.NewManager(lock.NewLocalLocker(disableLock))
	if err := locker.Acquire(cfg.Repo.Url); err != nil {
		return fmt.Errorf("lock acquire error: %w", err)
	}
	if disableLock {
		log.Println("Processing without execution lock")
	}

	gitCtx, cancelGit := context.WithTimeout(ctx, gitTimeout)
	defer cancelGit()
	gitClient, err := git.NewGitClient(gitCtx, *cfg.Repo)
	if err != nil {
		return fmt.Errorf("new git client error: %w", err)
	}

	gitTargetObject, err := gitClient.GetTargetGitObject(reference)
	if err != nil {
		return fmt.Errorf("get target git object error: %w", err)
	}

	repoTemplatevars := templates.GetRepoTemplateVars(templates.RepoTemplateVarsData{
		RepoTag:    gitTargetObject.Tag,
		RepoUrl:    cfg.Repo.Url,
		RepoCommit: gitTargetObject.Commit,
	})

	// Hooks get their own context: they must still be able to report after the
	// run context was cancelled by SIGTERM.
	hookExecutor, err := hooks.NewHookExecutor(context.Background(), cfg, hooks.HookExecutorOptions{
		TemplateVars: repoTemplatevars,
	})
	if err != nil {
		return fmt.Errorf("hooks executor error: %w", err)
	}

	if !disableQuorumsCheck {
		if err := quorum.CheckQuorums(&quorum.CheckQuorumsRequest{
			Quorums:      cfg.Quorums,
			Repo:         gitClient.Repo,
			Tag:          gitTargetObject.Tag,
			HookExecutor: hookExecutor,
		}); err != nil {
			return err
		}
	}

	storage, err := storage.NewStorage(&storage.StorageOpts{
		Config: cfg,
	})
	if err != nil {
		return fmt.Errorf("init storage error: %w", err)
	}

	taskExecutor, err := getExecutor(ctx, tasks.TaskExecutorOptions{
		Storage:      storage,
		TemplateVars: repoTemplatevars,
		WorkDir:      gitClient.RepoPath,
	})
	if err != nil {
		return fmt.Errorf("task executor error: %w", err)
	}

	tasksToRun, err := tasks.GetTasksToRun(cfg, gitClient.RepoPath, tasks.GetTasksToRunOpts{
		CmdFromCli:     opts.cmdFromCli,
		Forced:         force,
		TargetTaskName: task,
		Version:        gitTargetObject.Tag,
	})
	if err != nil {
		return fmt.Errorf("task executor error: %w", err)
	}

	// TODO: think about running this hook concurrently with the command
	for _, t := range tasksToRun {
		_ = hookExecutor.RunOnCommandStartedHook(t.Name)
	}

	if err := taskExecutor.RunTasks(tasksToRun); err != nil {
		return handleRunTasksError(err, hookExecutor)
	}

	_ = hookExecutor.RunOnCommandSuccessHook()
	return nil
}

func handleRunTasksError(err error, hookExecutor *hooks.HookExecutor) error {
	var runErr *tasks.Error
	if errors.As(err, &runErr) {
		switch {
		case errors.Is(runErr.Err, tasks.ErrNoNewVersion):
			_ = hookExecutor.RunOnCommandSkippedHook()
			log.Printf("task %s skipped: no new version detected\n", runErr.TaskName)
			return nil

		case errors.Is(runErr.Err, tasks.ErrExcutionFailed):
			_ = hookExecutor.RunOnCommandFailureHook(runErr.TaskName)
			return fmt.Errorf("task %s failed: %w", runErr.TaskName, runErr)

		case errors.Is(runErr.Err, tasks.ErrStateStoreFailed):
			// The release did run, only the state was lost. Report success to the
			// hooks and fail the run so that the operator can tell the difference.
			_ = hookExecutor.RunOnCommandSuccessHook()
			return fmt.Errorf("task %s succeeded, but the last processed tag was not stored, it will run again on the next poll: %w", runErr.TaskName, runErr)

		default:
			return fmt.Errorf("task running error: %w", runErr.Err)
		}
	}
	return fmt.Errorf("tasks running error: %w", err)
}

func getExecutor(ctx context.Context, opts tasks.TaskExecutorOptions) (Executor, error) {
	commonOpts := tasks.TaskExecutorOptions{
		Storage:      opts.Storage,
		TemplateVars: opts.TemplateVars,
		WorkDir:      opts.WorkDir,
	}
	if force {
		taskExecutor, err := tasks.NewTaskForceExecutor(ctx, commonOpts)
		if err != nil {
			return nil, fmt.Errorf("task executor error: %w", err)
		}
		return taskExecutor, nil
	} else {
		taskExecutor, err := tasks.NewTaskExecutor(ctx, commonOpts)
		if err != nil {
			return nil, fmt.Errorf("task executor error: %w", err)
		}
		return taskExecutor, nil
	}
}
