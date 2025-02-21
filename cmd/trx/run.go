package main

import (
	"errors"
	"fmt"
	"log"
	"time"
	"trx/internal/command"
	"trx/internal/config"
	"trx/internal/git"
	"trx/internal/lock"
	"trx/internal/quorum"
	"trx/internal/storage"
)

func run() error {
	log.SetFlags(0)
	log.Println("Running trx")
	log.Printf("Start at %s\n", time.Now().Format("2006-01-02 15:04:05"))

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

	t, err := getGitTargetObject(&cfg.Repo)
	if err != nil {
		return fmt.Errorf("git error: %w", err)
	}

	lastSucceedTag, err := storage.CheckLastSucceedTag()
	if err != nil {
		return fmt.Errorf("check last published commit error: %w", err)
	}

	executor, err := command.NewExecutor(cfg.Env, generateCmdVars(cfg, t))
	if err != nil {
		return fmt.Errorf("command executor error: %w", err)
	}

	isNewVersion, err := git.IsNewerVersion(t.Tag, lastSucceedTag, cfg.Repo.InitialLastProcessedTag)
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

	err = quorum.CheckQuorums(cfg.Quorums, t.Repository, t.Tag)
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

	var cmdsToRun []string
	if len(cfg.Commands) > 0 {
		cmdsToRun = cfg.Commands
	} else {
		runCfg, err := config.NewRunnerConfig(command.WorkDir, cfg.CommandsFilePath)
		if err != nil {
			return fmt.Errorf("config error: %w", err)
		}
		cmdsToRun = runCfg.Commands
	}

	if len(cmdsToRun) == 0 {
		return fmt.Errorf("no commands to run: %w", err)
	}

	go func() {
		log.Println("Running onCommandStarted hook")
		if hookErr := executor.RunOnCommandStartedHook(cfg); hookErr != nil {
			log.Println("WARNING onCommandStarted hook execution error: %w", hookErr)
		}
	}()

	if err := executor.Exec(cmdsToRun); err != nil {
		if hookErr := executor.RunOnCommandFailureHook(cfg); hookErr != nil {
			log.Println("WARNING onCommandFailure hook execution error: %w", hookErr)
		}
		return fmt.Errorf("run command error: %w", err)
	}

	if err := storage.StoreSucceedTag(t.Tag); err != nil {
		return fmt.Errorf("store last successed tag error: %w", err)
	}

	if hookErr := executor.RunOnCommandSuccessHook(cfg); hookErr != nil {
		log.Println("WARNING onCommandSuccess hook execution error: %w", hookErr)
	}

	log.Println("All done")
	return nil
}

func getGitTargetObject(cfg *config.GitRepo) (*git.TargetGitObject, error) {
	t, err := git.GetTargetGitObject(*cfg)
	if err != nil {
		return nil, fmt.Errorf("get target git object error: %w", err)
	}

	err = git.PerformCheckout(t.Repository, t.Tag)
	if err != nil {
		return nil, fmt.Errorf("checkout error: %w", err)
	}

	return t, nil
}

func generateCmdVars(cfg *config.Config, t *git.TargetGitObject) map[string]string {
	vars := make(map[string]string)
	vars["RepoTag"] = t.Tag
	vars["RepoUrl"] = cfg.Repo.Url
	vars["RepoCommit"] = t.Commit
	return vars
}
