package main

import (
	"errors"
	"fmt"
	"log"
	"trx/internal/command"
	"trx/internal/config"
	"trx/internal/git"
	"trx/internal/quorum"
	"trx/internal/storage"

	"github.com/spf13/cobra"
)

type Storage interface {
	CheckLastSuccessedTag() (string, error)
	StoreSuccessedTag(commit string) error
}

var configPath string

func main() {
	rootCmd := &cobra.Command{
		Use:   "trx",
		Short: "Runs quorum validation and runs specified command",
		Long: `trx is a tool for quorum verification and command execution in a Git repository.

By default, it uses the ./trx.yaml configuration file, but you can specify a different path using the --config flag.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := run(); err != nil {
				return err
			}
			return nil
		},
	}

	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "./trx.yaml", "Path to config file")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.SetFlags(0)
	log.Println("Running trx")

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	t, err := getGitTargerObject(&cfg.Repo)
	if err != nil {
		return fmt.Errorf("git error: %w", err)
	}

	var storage Storage = storage.NewLocalStorage(cfg.Repo.Url)
	lastSuccessedTag, err := storage.CheckLastSuccessedTag()
	if err != nil {
		return fmt.Errorf("check last published commit error: %w", err)
	}

	executor, err := command.NewExecutor(cfg.Env, generateCmdVars(cfg, t))
	if err != nil {
		return fmt.Errorf("command executor error: %w", err)
	}

	isNewVersion, err := git.IsNewerVersion(t.Tag, lastSuccessedTag, cfg.Repo.InitialLastprocessedTag)
	if err != nil {
		return fmt.Errorf("can't check if tag is new: %w", err)
	}
	if !isNewVersion {
		if hookErr := executor.RunOnCommandSkippedHook(cfg); hookErr != nil {
			log.Println("WARNING onCommandSkipped hook execution error: %w", hookErr)
		}
		return nil
	}

	err = quorum.CheckQuorums(cfg.Quorums, t.Repository, t.Tag)
	if err != nil {
		var qErr *quorum.QuorumError
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

	if len(cfg.Commands) > 0 {
		if err := executor.Exec(cfg.Commands); err != nil {
			if hookErr := executor.RunOnCommandFailureHook(cfg); hookErr != nil {
				log.Println("WARNING onCommandFailure hook execution error: %w", hookErr)
			}
			return fmt.Errorf("run command error: %w", err)
		}
	} else {
		runCfg, err := config.NewRunnerConfig(command.WorkDir, cfg.CommandsFilePath)
		if err != nil {
			return fmt.Errorf("config error: %w", err)
		}
		if err := executor.Exec(runCfg.Commands); err != nil {
			if hookErr := executor.RunOnCommandFailureHook(cfg); hookErr != nil {
				log.Println("WARNING onCommandFailure hook execution error: %w", hookErr)
			}
			return fmt.Errorf("run command error: %w", err)
		}
	}

	if err := storage.StoreSuccessedTag(t.Tag); err != nil {
		return fmt.Errorf("store last successed tag error: %w", err)
	}

	if hookErr := executor.RunOnCommandSuccessHook(cfg); hookErr != nil {
		log.Println("WARNING onCommandSuccess hook execution error: %w", hookErr)
	}

	log.Println("All done")
	return nil
}

func getGitTargerObject(cfg *config.GitRepo) (*git.TargetGitObject, error) {
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
