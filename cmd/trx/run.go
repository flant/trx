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
		// Restores the default disposition, so a second signal ends trx at
		// once instead of waiting for the shutdown hooks.
		signal.Stop(signalChan)
		cancel()
	}()

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	localLocker, err := lock.NewLocalLocker()
	if err != nil {
		return fmt.Errorf("locker error: %w", err)
	}

	locker := lock.NewManager(localLocker, disableLock, lockTimeout)
	if err := locker.Acquire(cfg.Repo.Url); err != nil {
		return fmt.Errorf("lock acquire error: %w", err)
	}

	// Under the lock: creating the storage may move state written by an
	// earlier version.
	storage, err := storage.NewStorage(&storage.StorageOpts{
		Config: cfg,
	})
	if err != nil {
		return fmt.Errorf("init storage error: %w", err)
	}

	gitClient, err := git.NewGitClient(ctx, cfg.Repo)
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
				log.Printf("WARNING onCommandSkipped hook execution error: %s", hookErr.Error())
			}
			log.Println("No new version. execution will be skipped")
			return nil
		}
	}

	// Up to here nothing has been checked out, and the executor still runs in
	// the directory trx was started in: the hooks above must not execute with
	// unverified repository content as their working directory.
	err = quorum.CheckQuorums(cfg.Quorums, gitClient.Repo, gitTargetObject.Tag)
	if err != nil {
		var qErr *quorum.Error
		if errors.As(err, &qErr) {
			executor.Vars["FailedQuorumName"] = qErr.QuorumName
			if hookErr := executor.RunOnQuorumFailedHook(cfg); hookErr != nil {
				log.Printf("WARNING onQuorumFailure hook execution error: %s", hookErr.Error())
			}
			return qErr
		} else {
			return fmt.Errorf("quorum error: %w", err)
		}
	}

	if err := gitClient.Checkout(gitTargetObject); err != nil {
		return fmt.Errorf("checkout error: %w", err)
	}
	executor.WorkDir = gitClient.RepoPath

	cmdsToRun, err := getCmdsToRun(cfg, opts, executor, gitClient.RepoPath)
	if err != nil {
		return fmt.Errorf("get commands to run error: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("interrupted before the commands were started: %w", err)
	}

	// TODO: think about running this hook concurrently with the command
	if hookErr := executor.RunOnCommandStartedHook(cfg); hookErr != nil {
		log.Printf("WARNING onCommandStarted hook execution error: %s", hookErr.Error())
	}

	if err := executor.Exec(cmdsToRun); err != nil {
		if hookErr := executor.RunOnCommandFailureHook(cfg); hookErr != nil {
			log.Printf("WARNING onCommandFailure hook execution error: %s", hookErr.Error())
		}
		return fmt.Errorf("run command error: %w", err)
	}

	if err := storage.StoreSucceedTag(gitTargetObject.Tag); err != nil {
		return fmt.Errorf("store last successed tag error: %w", err)
	}

	if hookErr := executor.RunOnCommandSuccessHook(cfg); hookErr != nil {
		log.Printf("WARNING onCommandSuccess hook execution error: %s", hookErr.Error())
	}

	log.Println("All done")
	return nil
}

// generateCmdVars predefines every documented variable, including the ones
// filled in later for a single hook: rendering fails on an unknown variable, so
// a shared env value mentioning FailedQuorumName would otherwise break every
// hook but one.
func generateCmdVars(cfg *config.Config, t *git.TargetGitObject) map[string]string {
	return map[string]string{
		"RepoTag":          t.Tag,
		"RepoUrl":          cfg.Repo.Url,
		"RepoCommit":       t.Commit,
		"FailedQuorumName": "",
	}
}

// mergeEnvs merges the environment of the repository config with the operator
// one, which wins as documented, into a new map: neither input is modified.
func mergeEnvs(repoEnv, operatorEnv map[string]string) map[string]string {
	merged := make(map[string]string, len(repoEnv)+len(operatorEnv))
	for k, v := range repoEnv {
		merged[k] = v
	}
	for k, v := range operatorEnv {
		merged[k] = v
	}
	return merged
}

func getCmdsToRun(cfg *config.Config, opts runOptions, executor *command.Executor, repoPath string) ([]string, error) {
	var cmdsToRun []string
	if len(opts.cmdFromCli) > 0 {
		cmdsToRun = []string{strings.Join(opts.cmdFromCli, " ")}
		return cmdsToRun, nil
	}

	if len(cfg.Commands) > 0 {
		cmdsToRun = cfg.Commands
	} else {
		runCfg, err := config.NewRunnerConfig(repoPath, cfg.Repo.ConfigFile)
		if err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
		cmdsToRun = runCfg.Commands
		executor.SetEnv(mergeEnvs(runCfg.Env, cfg.Env))
	}

	if len(cmdsToRun) == 0 {
		return nil, fmt.Errorf("no commands to run")
	}

	return cmdsToRun, nil
}
