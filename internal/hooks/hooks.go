package hooks

import (
	"context"
	"fmt"
	"log"

	"trx/internal/config"
	"trx/internal/executor"
	"trx/internal/templates"
)

type Executor interface {
	Exec(commands []string, env, templateVars map[string]string) error
}

type HookExecutor struct {
	hooks        config.Hooks
	executor     Executor
	env          map[string]string
	templateVars map[string]string
}

type HookExecutorOptions struct {
	TemplateVars map[string]string
}

// NewHookExecutor builds an executor that runs hooks in trx's own working
// directory. Hooks must never run in the repository worktree: onQuorumFailure
// would then execute in a checkout that failed verification.
func NewHookExecutor(ctx context.Context, cfg *config.Config, opts HookExecutorOptions) (*HookExecutor, error) {
	env := getEnv(cfg)
	hooks := cfg.Hooks
	e, err := executor.NewExecutor(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %v", err)
	}
	return &HookExecutor{
		hooks:        hooks,
		executor:     e,
		env:          env,
		templateVars: opts.TemplateVars,
	}, nil
}

func (e *HookExecutor) RunOnCommandStartedHook(taskname string) error {
	e.templateVars[templates.StartedTaskName] = taskname
	if e.hooks.OnCommandStarted != nil {
		log.Println("Running onCommandStarted hook")
		if err := e.executor.Exec(*e.hooks.OnCommandStarted, e.env, e.templateVars); err != nil {
			log.Printf("WARNING onCommandStarted hook execution error: %s\n", err.Error())
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnCommandSuccessHook() error {
	if e.hooks.OnCommandSuccess != nil {
		log.Println("Running onCommandSuccess hook")
		if err := e.executor.Exec(*e.hooks.OnCommandSuccess, e.env, e.templateVars); err != nil {
			log.Printf("WARNING onCommandSuccess hook execution error: %s\n", err.Error())
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnCommandFailureHook(taskname string) error {
	e.templateVars[templates.FailedTaskName] = taskname
	if e.hooks.OnCommandFailure != nil {
		log.Println("Running onCommandFailure hook")
		if err := e.executor.Exec(*e.hooks.OnCommandFailure, e.env, e.templateVars); err != nil {
			log.Printf("WARNING onCommandFailure hook execution error: %s\n", err.Error())
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnCommandSkippedHook() error {
	if e.hooks.OnCommandSkipped != nil {
		log.Println("Running onCommandSkipped hook")
		if err := e.executor.Exec(*e.hooks.OnCommandSkipped, e.env, e.templateVars); err != nil {
			log.Printf("WARNING onCommandSkipped hook execution error: %s\n", err.Error())
			return err
		}
	}
	return nil
}

func (e *HookExecutor) RunOnQuorumFailedHook(quorumName string) error {
	e.templateVars[templates.FailedQuorumName] = quorumName
	if e.hooks.OnQuorumFailure != nil {
		log.Println("Running onQuorumFailure hook")
		if err := e.executor.Exec(*e.hooks.OnQuorumFailure, e.env, e.templateVars); err != nil {
			log.Printf("WARNING onQuorumFailure hook execution error: %s\n", err.Error())
			return err
		}
	}
	return nil
}

func getEnv(cfg *config.Config) map[string]string {
	var hooksEnvs map[string]string
	if cfg.Env != nil {
		throwDeprWarning()
		hooksEnvs = cfg.Env
	} else {
		hooksEnvs = cfg.Hooks.Env
	}
	return hooksEnvs
}

func throwDeprWarning() {
	log.Println("WARNING! You're using deprecated 'env' field in config. Please use 'tasks.task.env' and 'hooks.env' instead.")
}
