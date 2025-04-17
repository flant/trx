package tasks

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"log"
	"strings"

	"trx/internal/config"
	"trx/internal/executor"
	"trx/internal/git"
)

type Executor interface {
	Exec(commands []string, env, templateVars map[string]string) error
}

type Storage interface {
	StoreTaskSucceedTag(taskName, commit string) error
	CheckTaskLastSucceedTag(taskName string) (string, error)
}

type TaskExecutor struct {
	executor     Executor
	templateVars map[string]string
	storage      Storage
}

type TaskExecutorForced struct {
	executor     Executor
	templateVars map[string]string
}

type RunOptions struct {
	CmdFromCli []string
}

type Task struct {
	Name           string
	Env            map[string]string
	Commands       []string
	Version        string
	InitialVersion string
}

type TaskExecutorOptions struct {
	Storage      Storage
	TemplateVars map[string]string
	WorkDir      string
}

type Error struct {
	TaskName   string
	Err        error
	ErrMessage string
}

var (
	ErrNoNewVersion   = errors.New("no new version")
	ErrExcutionFailed = errors.New("error running task")
)

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
	if opts.Storage == nil {
		return nil, fmt.Errorf("can't create executor: storage is required")
	}
	return &TaskExecutor{
		executor:     e,
		templateVars: opts.TemplateVars,
		storage:      opts.Storage,
	}, nil
}

func NewTaskForceExecutor(ctx context.Context, opts TaskExecutorOptions) (*TaskExecutorForced, error) {
	e, err := executor.NewExecutor(ctx, opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("can't create executor: %w", err)
	}
	return &TaskExecutorForced{
		executor:     e,
		templateVars: opts.TemplateVars,
	}, nil
}

func (e *TaskExecutorForced) RunTasks(tasks []Task) error {
	for _, t := range tasks {
		if err := e.executor.Exec(t.Commands, t.Env, e.templateVars); err != nil {
			return &Error{
				TaskName:   t.Name,
				Err:        ErrExcutionFailed,
				ErrMessage: err.Error(),
			}
		}
	}
	return nil
}

func (e *TaskExecutor) RunTasks(tasks []Task) error {
	for _, t := range tasks {
		if err := t.checkIfNewVersion(e.storage); err != nil {
			return &Error{
				TaskName: t.Name,
				Err:      ErrNoNewVersion,
			}
		}
		if err := e.executor.Exec(t.Commands, t.Env, e.templateVars); err != nil {
			return &Error{
				TaskName:   t.Name,
				Err:        ErrExcutionFailed,
				ErrMessage: err.Error(),
			}
		}
		if err := e.storage.StoreTaskSucceedTag(t.Name, t.Version); err != nil {
			return fmt.Errorf("store last successed tag error for task %s: %w", t.Name, err)
		}
	}
	return nil
}

func (t *Task) checkIfNewVersion(storage Storage) error {
	lastSucceedTag, err := storage.CheckTaskLastSucceedTag(t.Name)
	if err != nil {
		return fmt.Errorf("check last published commit error: %w", err)
	}
	isNewVersion, err := git.IsNewerVersion(t.Version, lastSucceedTag, t.InitialVersion)
	if err != nil {
		return fmt.Errorf("can't check if tag is new: %w", err)
	}
	if !isNewVersion {
		return fmt.Errorf("no new version")
	}
	return nil
}

type GetTasksToRunOpts struct {
	CmdFromCli     []string
	Forced         bool
	TargetTaskName string
	Version        string
}

func GetTasksToRun(cfg *config.Config, wd string, opts GetTasksToRunOpts) ([]Task, error) {
	if len(cfg.Commands) > 0 {
		throwDeprWarning()
		return []Task{
			{
				Name:           "legacy-commands-main",
				Commands:       cfg.Commands,
				Env:            cfg.Env,
				Version:        opts.Version,
				InitialVersion: cfg.Repo.InitialLastProcessedTag,
			},
		}, nil
	}
	if len(opts.CmdFromCli) > 0 {
		cmd := []string{strings.Join(opts.CmdFromCli, " ")}
		return []Task{
			{
				Name:     GetHashOfCommands(cmd),
				Commands: cmd,
				Version:  opts.Version,
			},
		}, nil
	}

	if len(cfg.Tasks) > 0 {
		return getTaskToRun(cfg.Tasks, opts.TargetTaskName, opts.Version)
	}

	runCfg, err := config.NewRunnerConfig(wd, cfg.Repo.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("runner config error: %w", err)
	}

	if len(runCfg.Commands) > 0 {
		throwDeprWarning()
		return []Task{
			{
				Name:           "legacy-commands",
				Commands:       runCfg.Commands,
				Env:            runCfg.Env,
				Version:        opts.Version,
				InitialVersion: cfg.Repo.InitialLastProcessedTag,
			},
		}, nil
	}
	return getTaskToRun(runCfg.Tasks, opts.TargetTaskName, opts.Version)
}

func throwDeprWarning() {
	deprWarning := "WARNING! You're using deprecated 'commands' field in config. Please use 'tasks' instead."
	log.Println(deprWarning)
}

func GetHashOfCommands(commands []string) string {
	joined := strings.Join(commands, "")
	sum := sha1.Sum([]byte(joined))
	return fmt.Sprintf("%x", sum)
}

func getTaskToRun(c []config.Task, name, version string) ([]Task, error) {
	if len(c) == 0 {
		return nil, fmt.Errorf("no tasks found")
	}
	if name == "" {
		return []Task{
			{
				Name: func() string {
					if c[0].Name != "" {
						return c[0].Name
					}
					return "1"
				}(),
				Commands:       c[0].Commands,
				Env:            c[0].Env,
				Version:        version,
				InitialVersion: c[0].InitialLastProcessedTag,
			},
		}, nil
	}

	for _, t := range c {
		if t.Name == name {
			return []Task{
				{
					Name:           t.Name,
					Commands:       t.Commands,
					Env:            t.Env,
					Version:        version,
					InitialVersion: t.InitialLastProcessedTag,
				},
			}, nil

		}

	}
	return nil, fmt.Errorf("task `%s` not found", name)
}
