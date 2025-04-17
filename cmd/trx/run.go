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
		return fmt.Errorf("config error: %w", err)
	}

	storage, err := storage.NewStorage(&storage.StorageOpts{
		Config: cfg,
	})
	if err != nil {
		return fmt.Errorf("init storage error: %w", err)
	}

	locker := lock.NewManager(lock.NewLocalLocker(disableLock))
	if err := locker.Acquire(cfg.Repo.Url); err != nil {
		return fmt.Errorf("lock acquire error: %w", err)
	}
	if disableLock {
		log.Println("Processing without execution lock")
	}

	gitClient, err := git.NewGitClient(cfg.Repo)
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

	hookExecutor, err := hooks.NewHookExecutor(ctx, cfg, hooks.HookExecutorOptions{
		TemplateVars: repoTemplatevars,
		WorkDir:      gitClient.RepoPath,
	})
	if err != nil {
		return fmt.Errorf("hooks executor error: %w", err)
	}

	if !disableQuorumsCheck {
		if err := quorum.CheckQuorums(cfg.Quorums, gitClient.Repo, gitTargetObject.Tag); err != nil {
			var qErr *quorum.Error
			if errors.As(err, &qErr) {
				hookExecutor.RunOnQuorumFailedHook(qErr.QuorumName)
				return fmt.Errorf("quorum error: %w", qErr.Err)
			} else {
				return fmt.Errorf("quorum error: %w", err)
			}
		}
	}

	taskExecutor, err := tasks.NewTaskExecutor(ctx, tasks.TaskExecutorOptions{
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
	hookExecutor.RunOnCommandStartedHook()

	if err := taskExecutor.RunTasks(tasksToRun); err != nil {
		return handleRunTasksError(err, hookExecutor)
	}

	hookExecutor.RunOnCommandSuccessHook()

	log.Println("All done")
	return nil
}

func handleRunTasksError(err error, hookExecutor *hooks.HookExecutor) error {
	var runErr *tasks.Error
	if errors.As(err, &runErr) {
		switch {
		case errors.Is(runErr.Err, tasks.ErrNoNewVersion):
			hookExecutor.RunOnCommandSkippedHook()
			return fmt.Errorf("task [%s] skipped: no new version detected", runErr.TaskName)

		case errors.Is(runErr.Err, tasks.ErrExcutionFailed):
			hookExecutor.RunOnCommandFailureHook(runErr.TaskName)
			return fmt.Errorf("tasks [%s] error: %w", runErr.TaskName, runErr.Err)
		default:
			return fmt.Errorf("task running error: %w", runErr.Err)
		}
	}
	return fmt.Errorf("tasks running error: %w", err)
}
