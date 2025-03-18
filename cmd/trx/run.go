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
	"trx/internal/command"
	"trx/internal/config"
	"trx/internal/git"
	"trx/internal/lock"
	"trx/internal/quorum"
	"trx/internal/storage"
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

	if !disableLock {
		locker := lock.NewLocker(storage, disableLock)
		if err := locker.CheckLock(); err != nil {
			var locked *lock.ErrLocked
			if errors.As(err, &locked) {
				log.Printf("Execution is locked by %s at %s\n", locked.User, locked.CreatedAt)
				return nil
			}
			return fmt.Errorf("check lock error: %w", err)
		}
		err := locker.Lock()
		if err != nil {
			return fmt.Errorf("lock error: %w", err)
		}
		log.Println("Excution lock acquired")
		defer func() {
			err := locker.Unlock()
			if err != nil {
				log.Println("Unlock error: %w", err)
			}
			log.Println("Execution lock released")
		}()
	}

	gitClient, err := git.NewGitClient(cfg.Repo)
	if err != nil {
		return fmt.Errorf("new git client error: %w", err)
	}

	gitTargetObject, err := gitClient.GetTargetGitObject()
	if err != nil {
		return fmt.Errorf("get target git object error: %w", err)
	}

	lastSucceedTag, err := storage.CheckLastSucceedTag()
	if err != nil {
		return fmt.Errorf("check last published commit error: %w", err)
	}

	executor, err := command.NewExecutor(ctx, cfg.Env, generateCmdVars(cfg, gitTargetObject))
	if err != nil {
		return fmt.Errorf("command executor error: %w", err)
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
			if hookErr := executor.RunOnCommandSkippedHook(cfg); hookErr != nil {
				log.Println("WARNING onCommandSkipped hook execution error: %w", hookErr)
			}
			log.Println("No new version. execution will be skipped")
			return nil
		}
	}

	err = quorum.CheckQuorums(cfg.Quorums, gitClient.Repo, gitTargetObject.Tag)
	if err != nil {
		var qErr *quorum.Error
		if errors.As(err, &qErr) {
			executor.Vars["FailedQuorumName"] = qErr.QuorumName
			if hookErr := executor.RunOnQuorumFailedHook(cfg); hookErr != nil {
				log.Println("WARNING onCommandSkipped hook execution error: %w", hookErr)
			}
			return fmt.Errorf("quorum error: %w", qErr.Err)
		} else {
			return fmt.Errorf("quorum error: %w", err)
		}
	}

	cmdsToRun, err := getCmdsToRun(cfg, opts, executor)
	if err != nil {
		return fmt.Errorf("get commands to run error: %w", err)
	}

	// TODO: think about running this hook concurrently with the command
	if hookErr := executor.RunOnCommandStartedHook(cfg); hookErr != nil {
		log.Printf("WARNING onCommandStarted hook execution error: %s", hookErr.Error())
	}

	if err := executor.Exec(cmdsToRun); err != nil {
		if hookErr := executor.RunOnCommandFailureHook(cfg); hookErr != nil {
			log.Println("WARNING onCommandFailure hook execution error: %w", hookErr)
		}
		return fmt.Errorf("run command error: %w", err)
	}

	if err := storage.StoreSucceedTag(gitTargetObject.Tag); err != nil {
		return fmt.Errorf("store last successed tag error: %w", err)
	}

	if hookErr := executor.RunOnCommandSuccessHook(cfg); hookErr != nil {
		log.Println("WARNING onCommandSuccess hook execution error: %w", hookErr)
	}

	log.Println("All done")
	return nil
}

func generateCmdVars(cfg *config.Config, t *git.TargetGitObject) map[string]string {
	vars := make(map[string]string)
	vars["RepoTag"] = t.Tag
	vars["RepoUrl"] = cfg.Repo.Url
	vars["RepoCommit"] = t.Commit
	return vars
}

func mergeEnvs(envs map[string]string, cfgEnv map[string]string) []string {
	for k, v := range cfgEnv {
		envs[k] = v
	}
	newEnv := make([]string, 0, len(envs))
	for k, v := range envs {
		newEnv = append(newEnv, fmt.Sprintf("%s=%s", k, v))
	}
	return newEnv
}

func getCmdsToRun(cfg *config.Config, opts runOptions, executor *command.Executor) ([]string, error) {
	var cmdsToRun []string
	if len(opts.cmdFromCli) > 0 {
		cmdsToRun = []string{strings.Join(opts.cmdFromCli, " ")}
		return cmdsToRun, nil
	}

	if len(cfg.Commands) > 0 {
		cmdsToRun = cfg.Commands
	} else {
		runCfg, err := config.NewRunnerConfig(command.WorkDir, cfg.Repo.ConfigFile)
		if err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
		cmdsToRun = runCfg.Commands
		executor.Env = mergeEnvs(cfg.Env, runCfg.Env)
	}

	if len(cmdsToRun) == 0 {
		return nil, fmt.Errorf("no commands to run")
	}

	return cmdsToRun, nil
}
