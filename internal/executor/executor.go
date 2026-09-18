package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/template"
)

// stderrTailLimit caps how much stderr is kept in memory for the error message.
const stderrTailLimit = 64 * 1024

type Executor struct {
	Ctx     context.Context
	WorkDir string
}

func NewExecutor(ctx context.Context, workDir string) (*Executor, error) {
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("unable to get working directory: %w", err)
		}
		workDir = wd
	}
	return &Executor{
		Ctx:     ctx,
		WorkDir: workDir,
	}, nil
}

func (e *Executor) Exec(commands []string, env, templateVars map[string]string) error {
	opts, err := prepareExecOpts(e.WorkDir, commands, env, templateVars)
	if err != nil {
		return fmt.Errorf("can't prepare exec opts: %w", err)
	}
	if err := execute(e.Ctx, opts); err != nil {
		return fmt.Errorf("executor error: %w", err)
	}
	return nil
}

func prepareExecOpts(wd string, commands []string, env, templateVars map[string]string) (*excuteOpts, error) {
	var envs []string
	for k, v := range env {
		envs = append(envs, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
	}
	cmds, err := resolve(commands, templateVars)
	if err != nil {
		return nil, fmt.Errorf("can't resolve commands: %w", err)
	}
	envs, err = resolve(envs, templateVars)
	if err != nil {
		return nil, fmt.Errorf("can't resolve envs: %w", err)
	}
	script := "set -e\n" + strings.Join(cmds, "\n")

	return &excuteOpts{
		cmd: script,
		env: envs,
		wd:  wd,
	}, nil
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
	// missingkey=error: an unknown variable must not silently expand into a
	// shell command.
	t, err := template.New("cmd").Option("missingkey=error").Parse(tmpl)
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
	cmd := exec.Command("sh", "-c", opts.cmd)
	cmd.Dir = opts.wd
	cmd.Env = append(os.Environ(), opts.env...)
	setProcessGroup(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %w", err)
	}

	// Signal the whole process group, otherwise only `sh` dies and the actual
	// workload keeps running as an orphan.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			log.Println("Terminating running command")
			terminateProcessGroup(cmd)
		case <-done:
		}
	}()

	var (
		tail tail
		wg   sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		stream(stdoutPipe, nil)
	}()
	go func() {
		defer wg.Done()
		stream(stderrPipe, &tail)
	}()
	// Both pipes must be drained before Wait, which closes them.
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if t := tail.String(); t != "" {
			return fmt.Errorf("error executing command: %w\n%s", err, t)
		}
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}

// stream logs the output line by line without a line length limit and keeps the
// last stderrTailLimit bytes for the error message.
func stream(r io.Reader, tail *tail) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			log.Println(strings.TrimRight(line, "\n"))
			if tail != nil {
				tail.append(line)
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("error reading command output: %s", err)
			}
			return
		}
	}
}

type tail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tail) append(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, s...)
	if len(t.buf) > stderrTailLimit {
		t.buf = t.buf[len(t.buf)-stderrTailLimit:]
	}
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimRight(string(t.buf), "\n")
}
