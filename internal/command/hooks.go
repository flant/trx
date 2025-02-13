package command

import (
	"log"
	"trx/internal/config"
)

func (e *Executor) RunOnCommandSuccessHook(cfg *config.Config) error {
	if cfg.Hooks.OnCommandSuccess != nil {
		log.Println("Running onCommandSuccess hook")
		if err := e.Exec(*cfg.Hooks.OnCommandSuccess); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) RunOnCommandFailureHook(cfg *config.Config) error {
	if cfg.Hooks.OnCommandFailure != nil {
		log.Println("Running onCommandFailure hook")
		if err := e.Exec(*cfg.Hooks.OnCommandFailure); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) RunOnCommandSkippedHook(cfg *config.Config) error {
	if cfg.Hooks.OnCommandSkiped != nil {
		log.Println("Running onCommandSkiped hook")
		if err := e.Exec(*cfg.Hooks.OnCommandSkiped); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) RunOnQuorumFailedHook(cfg *config.Config) error {
	if cfg.Hooks.OnQuorumFailure != nil {
		log.Println("Running onQuorumFailure hook")
		if err := e.Exec(*cfg.Hooks.OnQuorumFailure); err != nil {
			return err
		}
	}
	return nil
}
