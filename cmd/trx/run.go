package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
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

	lastSucceedTag, err := storage.CheckLastSucceedTag()
	if err != nil {
		return fmt.Errorf("check last published commit error: %w", err)
	}

	repoTemplatevars := templates.GetRepoTemplateVars(templates.RepoTemplateVarsData{
		RepoTag:    gitTargetObject.Tag,
		RepoUrl:    cfg.Repo.Url,
		RepoCommit: gitTargetObject.Commit,
	})

	hookExecutor, err := hooks.NewHookExecutor(ctx, hooks.HookExecutorOptions{
		Env:          cfg.Hooks.Env,
		TemplateVars: repoTemplatevars,
		WorkDir:      gitClient.RepoPath,
	})
	if err != nil {
		return fmt.Errorf("hooks executor error: %w", err)
	}

	isNewVersion, err := git.IsNewerVersion(gitTargetObject.Tag, lastSucceedTag, cfg.Repo.InitialLastProcessedTag)
	if err != nil {
		return fmt.Errorf("can't check if tag is new: %w", err)
	}
	if !isNewVersion {
		switch force {
		case true:
			log.Println("No new version, but force flag specified. Proceeding... ")
		case false:
			if hookErr := hookExecutor.RunOnCommandSkippedHook(cfg); hookErr != nil {
				log.Println("WARNING onCommandSkipped hook execution error: %w", hookErr)
			}
			log.Println("No new version. execution will be skipped")
			return nil
		}
	}

	if !disableQuorumsCheck {
		if err := quorum.CheckQuorums(cfg.Quorums, gitClient.Repo, gitTargetObject.Tag); err != nil {
			var qErr *quorum.Error
			if errors.As(err, &qErr) {
				if hookErr := hookExecutor.RunOnQuorumFailedHook(cfg, qErr.QuorumName); hookErr != nil {
					log.Println("WARNING onCommandSkipped hook execution error: %w", hookErr)
				}
				return fmt.Errorf("quorum error: %w", qErr.Err)
			} else {
				return fmt.Errorf("quorum error: %w", err)
			}
		}
	}

	taskExecutor, err := tasks.NewTaskExecutor(ctx, tasks.TaskExecutorOptions{
		TemplateVars: repoTemplatevars,
		WorkDir:      gitClient.RepoPath,
	})
	if err != nil {
		return fmt.Errorf("task executor error: %w", err)
	}

	tasksToRun, err := getTasksToRun(cfg, gitClient.RepoPath, opts)
	if err != nil {
		return fmt.Errorf("task executor error: %w", err)
	}

	// TODO: think about running this hook concurrently with the command
	if hookErr := hookExecutor.RunOnCommandStartedHook(cfg); hookErr != nil {
		log.Printf("WARNING onCommandStarted hook execution error: %s", hookErr.Error())
	}

	if err := taskExecutor.RunTasks(tasksToRun); err != nil {
		var runErr *tasks.Error
		if errors.As(err, &runErr) {
			if hookErr := hookExecutor.RunOnCommandFailureHook(cfg, runErr.TaskName); hookErr != nil {
				log.Println("WARNING onCommandFailure hook execution error: %w", hookErr)
			}
			return fmt.Errorf("tasks running error: %w", runErr.Err)
		} else {
			return fmt.Errorf("tasks running error: %w", err)
		}
	}

	if err := storage.StoreSucceedTag(gitTargetObject.Tag); err != nil {
		return fmt.Errorf("store last successed tag error: %w", err)
	}

	if hookErr := hookExecutor.RunOnCommandSuccessHook(cfg); hookErr != nil {
		log.Println("WARNING onCommandSuccess hook execution error: %w", hookErr)
	}

	log.Println("All done")
	return nil
}

func getTasksToRun(cfg *config.Config, wd string, opts runOptions) ([]tasks.Task, error) {
	if len(opts.cmdFromCli) > 0 {
		return []tasks.Task{
			{
				Name:     "command-from-cli",
				Commands: []string{strings.Join(opts.cmdFromCli, " ")},
			},
		}, nil
	}

	if len(cfg.Tasks) > 0 {
		return cfgToTasks(cfg.Tasks), nil
	}

	runCfg, err := config.NewRunnerConfig(wd, cfg.Repo.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	tasksList := cfgToTasks(runCfg.Tasks)

	if len(tasksList) == 0 {
		return nil, fmt.Errorf("no commands to run")
	}

	return tasksList, nil
}

func cfgToTasks(c []config.Task) []tasks.Task {
	res := []tasks.Task{}
	for i, t := range c {
		tsk := tasks.Task{
			Name:     t.Name,
			Env:      t.Env,
			Commands: t.Commands,
		}
		if tsk.Name == "" {
			tsk.Name = fmt.Sprint(i)
		}
		res = append(res, tsk)
	}
	return res
}
