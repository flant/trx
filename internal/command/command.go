package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
)

type Vars struct {
	RepoUrl string
	RepoTag string
}

type Executor struct {
	Ctx     context.Context
	WorkDir string
	Env     []string
	Vars    map[string]string
}

// NewExecutor returns an executor running in the working directory trx itself
// was started in. Only after the tag has passed quorum verification and has
// been checked out may WorkDir be moved to the clone.
func NewExecutor(ctx context.Context, e, vars map[string]string) (*Executor, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unable to determine the working directory: %w", err)
	}
	return &Executor{
		Ctx:     ctx,
		WorkDir: wd,
		Env:     envSlice(e),
		Vars:    vars,
	}, nil
}

// SetEnv replaces the environment of the executor.
func (e *Executor) SetEnv(envs map[string]string) {
	e.Env = envSlice(envs)
}

// envSlice renders environment variables as KEY=value. Viper lower-cases every
// key it reads, so the names are upper-cased here, in the one place both the
// operator config and the repository config go through.
func envSlice(envs map[string]string) []string {
	out := make([]string, 0, len(envs))
	for k, v := range envs {
		out = append(out, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
	}
	sort.Strings(out)
	return out
}

func (e *Executor) Exec(commands []string) error {
	cmds, err := resolve(commands, e.Vars)
	if err != nil {
		return fmt.Errorf("can't resolve commands: %w", err)
	}
	envs, err := resolve(e.Env, e.Vars)
	if err != nil {
		return fmt.Errorf("can't resolve envs: %w", err)
	}
	script := "set -e\n" + strings.Join(cmds, "\n")
	if err := execute(e.Ctx, &excuteOpts{
		cmd: script,
		env: envs,
		wd:  e.WorkDir,
	}); err != nil {
		return fmt.Errorf("executor error: %w", err)
	}
	return nil
}

func resolve(commands []string, vars map[string]string) ([]string, error) {
	resolved := make([]string, len(commands))
	for i, cmd := range commands {
		resCmd, err := resolveTemplate(cmd, vars)
		if err != nil {
			return nil, err
		}
		resolved[i] = resCmd
	}
	return resolved, nil
}

func resolveTemplate(tmpl string, vars map[string]string) (string, error) {
	t, err := template.New("cmd").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

type excuteOpts struct {
	cmd string
	env []string
	wd  string
}

func execute(ctx context.Context, opts *excuteOpts) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", opts.cmd)
	cmd.Dir = opts.wd
	cmd.Env = append(os.Environ(), opts.env...)

	// Both streams are copied to the log as the command produces them:
	// os/exec owns the copying goroutines and waits for them before Run
	// returns, so no output can be lost and no amount of output can block
	// the child.
	out := &syncWriter{w: log.Writer()}
	cmd.Stdout = out
	cmd.Stderr = out

	// A backgrounded grandchild keeps the pipes open after sh itself is
	// gone, which would hold the execution lock forever.
	cmd.WaitDelay = outputWaitDelay

	if err := cmd.Run(); err != nil {
		// The command itself finished: only a process it left behind kept
		// the output open, which is not a reason to report a failure.
		if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.Success() {
			log.Println("WARNING a background process is still holding the command output open")
			return nil
		}
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}

// outputWaitDelay is how long the output of a process outliving the command is
// still copied. A variable so the tests do not have to wait for it.
var outputWaitDelay = 10 * time.Second

// syncWriter serializes the stdout and stderr copying goroutines.
//
// os/exec runs one goroutine per stream unless both writers are the same
// comparable value, which is why the same syncWriter is used for both: a
// pointer compares equal to itself, so in practice only one goroutine writes
// and the mutex is a guard against that guarantee changing.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}
