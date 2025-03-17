package command

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

var WorkDir = ""

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

func NewExecutor(ctx context.Context, e, vars map[string]string) (*Executor, error) {
	wd := WorkDir
	if wd == "" {
		wd, _ = os.Getwd()
	}
	var envs []string
	for k, v := range e {
		envs = append(envs, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
	}
	return &Executor{
		Ctx:     ctx,
		WorkDir: wd,
		Env:     envs,
		Vars:    vars,
	}, nil
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

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	defer stdoutPipe.Close()

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	defer stderrPipe.Close()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			log.Println(scanner.Text())
		}
	}()

	var stderr bytes.Buffer
	if _, err := io.Copy(&stderr, stderrPipe); err != nil {
		log.Printf("error write stderr buffer: %s", err.Error())
	}

	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			log.Println("executing error:", stderr.String())
		}
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}
