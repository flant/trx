package executor

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

type Vars struct {
	RepoUrl string
	RepoTag string
}

type Executor struct {
	Ctx     context.Context
	WorkDir string
}

func NewExecutor(ctx context.Context, workDir string) (*Executor, error) {
	if workDir == "" {
		workDir, _ = os.Getwd()
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
