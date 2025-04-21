package executor

import (
	"context"
	"fmt"
)

type MockExecutor struct {
	Ctx     context.Context
	WorkDir string
}

func NewMockExecutor(ctx context.Context, workDir string) (*MockExecutor, error) {
	if workDir == "" {
		workDir = "/mock/work/dir"
	}
	return &MockExecutor{
		Ctx:     ctx,
		WorkDir: workDir,
	}, nil
}

func (e *MockExecutor) Exec(commands []string, env, templateVars map[string]string) error {
	opts, err := prepareExecOpts(e.WorkDir, commands, env, templateVars)
	if err != nil {
		return fmt.Errorf("can't prepare exec opts: %w", err)
	}
	if err := executeMock(e.Ctx, opts); err != nil {
		return fmt.Errorf("executor error: %w", err)
	}
	return nil
}

func executeMock(_ context.Context, opts *excuteOpts) error {
	fmt.Printf("Mock executing commands: %s\n", opts.cmd)
	return nil
}
