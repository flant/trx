package command

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
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
	envs := []string{}
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
	cmds, err := resolveCommands(commands, e.Vars)
	if err != nil {
		return fmt.Errorf("can't resolve commands: %w", err)
	}
	for _, c := range cmds {
		err := execute(c, e.Env, e.WorkDir)
		if err != nil {
			return fmt.Errorf("error execute command %s: %w", c, err)
		}
	}
	return nil
}

func resolveCommands(commands []string, vars map[string]string) ([]string, error) {
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

type Command struct {
	Command string
	Args    []string
	WorkDir string
	Env     []string
}

func NewCommand(c []string, env map[string]string) *Command {
	if WorkDir == "" {
		WorkDir, _ = os.Getwd()
	}
	log.Printf("Trying to run command %v\n", c)
	envs := []string{}
	for k, v := range env {
		envs = append(envs, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
	}
	return &Command{
		Command: strings.Join(c, " "),
		WorkDir: WorkDir,
		Env:     envs,
	}
}

func (c *Command) Exec() error {
	return execute(c.Command, c.Env, c.WorkDir)
}

func execute(command string, env []string, wd string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = wd
	cmd.Env = append(os.Environ(), env...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
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

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}
