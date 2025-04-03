package tasks

import (
	"context"
	"fmt"
	"trx/internal/executor"
)

type Executor interface {
	Exec(commands []string, env, templateVars map[string]string) error
}

type TaskExecutor struct {
	executor     Executor
	templateVars map[string]string
}

type RunOptions struct {
	CmdFromCli []string
}

type Task struct {
	Name     string
	Env      map[string]string
	Commands []string
}

type TaskExecutorOptions struct {
	TemplateVars map[string]string
	WorkDir      string
}

type Error struct {
	TaskName string
	Err      error
}

func (e *Error) Error() string {
	return fmt.Sprintf("task `%s` error: %v", e.TaskName, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewTaskExecutor(ctx context.Context, opts TaskExecutorOptions) (*TaskExecutor, error) {
	e, err := executor.NewExecutor(ctx, opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("can't create executor: %w", err)
	}
	return &TaskExecutor{
		executor: e,
	}, nil
}

func (e *TaskExecutor) RunTasks(tasks []Task) error {
	for _, t := range tasks {
		if err := e.executor.Exec(t.Commands, t.Env, e.templateVars); err != nil {
			return &Error{
				TaskName: t.Name,
				Err:      err,
			}
		}
	}
	return nil
}
