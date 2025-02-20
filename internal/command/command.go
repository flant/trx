package command

import (
	"bufio"
	"bytes"
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
	WorkDir string
	Env     []string
	Vars    map[string]string
}

func NewExecutor(e map[string]string, vars map[string]string) (*Executor, error) {
	wd := WorkDir
	if wd == "" {
		wd, _ = os.Getwd()
	}
	var envs []string
	for k, v := range e {
		envs = append(envs, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
	}
	return &Executor{
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
	for _, c := range cmds {
		err := execute(c, envs, e.WorkDir)
		if err != nil {
			return fmt.Errorf("error execute command %s: %w", c, err)
		}
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

func execute(command string, env []string, wd string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = wd
	cmd.Env = append(os.Environ(), env...)
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
