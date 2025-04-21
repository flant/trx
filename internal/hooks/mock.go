package hooks

import (
	"context"
	"fmt"
	"trx/internal/config"
	"trx/internal/executor"
)

func NewHookMockExecutor(ctx context.Context, cfg *config.Config, opts HookExecutorOptions) (*HookExecutor, error) {
	env := getEnv(cfg)
	hooks := cfg.Hooks
	e, err := executor.NewMockExecutor(ctx, opts.WorkDir)
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
