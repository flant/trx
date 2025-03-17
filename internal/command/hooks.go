package command

import (
	"log"

	"trx/internal/config"
)

func (e *Executor) RunOnCommandStartedHook(cfg *config.Config) error {
	if cfg.Hooks != nil && cfg.Hooks.OnCommandStarted != nil {
		if err := e.Exec(*cfg.Hooks.OnCommandStarted); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) RunOnCommandSuccessHook(cfg *config.Config) error {
	if cfg.Hooks != nil && cfg.Hooks.OnCommandSuccess != nil {
		log.Println("Running onCommandSuccess hook")
		if err := e.Exec(*cfg.Hooks.OnCommandSuccess); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) RunOnCommandFailureHook(cfg *config.Config) error {
	if cfg.Hooks != nil && cfg.Hooks.OnCommandFailure != nil {
		log.Println("Running onCommandFailure hook")
		if err := e.Exec(*cfg.Hooks.OnCommandFailure); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) RunOnCommandSkippedHook(cfg *config.Config) error {
	if cfg.Hooks != nil && cfg.Hooks.OnCommandSkipped != nil {
		log.Println("Running onCommandSkipped hook")
		if err := e.Exec(*cfg.Hooks.OnCommandSkipped); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) RunOnQuorumFailedHook(cfg *config.Config) error {
	if cfg.Hooks != nil && cfg.Hooks.OnQuorumFailure != nil {
		log.Println("Running onQuorumFailure hook")
		if err := e.Exec(*cfg.Hooks.OnQuorumFailure); err != nil {
			return err
		}
	}
	return nil
}
