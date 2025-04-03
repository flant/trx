package hooks

import (
	"context"
	"log"

	"trx/internal/config"
	"trx/internal/executor"
	"trx/internal/templates"
)

type Executor interface {
	Exec(commands []string, env, templateVars map[string]string) error
}

type HookExecutor struct {
	executor     Executor
	env          map[string]string
	templateVars map[string]string
}

type HookExecutorOptions struct {
	Env          map[string]string
	TemplateVars map[string]string
	WorkDir      string
}

func NewHookExecutor(ctx context.Context, opts HookExecutorOptions) (*HookExecutor, error) {
	e, err := executor.NewExecutor(ctx, opts.WorkDir)
	if err != nil {
		log.Fatalf("failed to create executor: %v", err)
	}
	return &HookExecutor{
		executor:     e,
		env:          opts.Env,
		templateVars: opts.TemplateVars,
	}, nil
}

func (e *HookExecutor) RunOnCommandStartedHook(cfg *config.Config) error {
	if cfg.Hooks.OnCommandStarted != nil {
		log.Println("Running onStartedSuccess hook")
		if err := e.executor.Exec(*cfg.Hooks.OnCommandStarted, e.env, e.templateVars); err != nil {
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnCommandSuccessHook(cfg *config.Config) error {
	if cfg.Hooks.OnCommandSuccess != nil {
		log.Println("Running onCommandSuccess hook")
		if err := e.executor.Exec(*cfg.Hooks.OnCommandSuccess, e.env, e.templateVars); err != nil {
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnCommandFailureHook(cfg *config.Config, taskname string) error {
	e.templateVars[templates.FailedTaskName] = taskname
	if cfg.Hooks.OnCommandFailure != nil {
		log.Println("Running onCommandFailure hook")
		if err := e.executor.Exec(*cfg.Hooks.OnCommandFailure, e.env, e.templateVars); err != nil {
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnCommandSkippedHook(cfg *config.Config) error {
	if cfg.Hooks.OnCommandSkipped != nil {
		log.Println("Running onCommandSkipped hook")
		if err := e.executor.Exec(*cfg.Hooks.OnCommandSkipped, e.env, e.templateVars); err != nil {
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnQuorumFailedHook(cfg *config.Config, quorumName string) error {
	e.templateVars[templates.FailedQuorumName] = quorumName
	if cfg.Hooks.OnQuorumFailure != nil {
		log.Println("Running onQuorumFailure hook")
		if err := e.executor.Exec(*cfg.Hooks.OnQuorumFailure, e.env, e.templateVars); err != nil {
			return err
		}
	}
	return nil
}
