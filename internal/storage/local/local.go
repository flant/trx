package storage

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"trx/internal/git"
)

const (
	fileLastProcessedTag = "last_processed_tag"
)

type Local struct {
	path string
}

func NewLocalStorage(repoUrl string) *Local {
	usr, _ := user.Current()
	repoName := git.RepoNameFromUrl(repoUrl)
	return &Local{
		path: filepath.Join(usr.HomeDir, ".trx", "storage", repoName),
	}
}

func (s *Local) CheckTaskLastSucceedTag(taskName string) (string, error) {
	if taskName == "" {
		return "", fmt.Errorf("task name can't be empty")
	}
	path := filepath.Join(s.path, taskName)
	filePath := filepath.Join(path, fileLastProcessedTag)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("error read from local storage: %w", err)
	}

	tag := strings.TrimSpace(string(data))
	if tag == "" {
		return "", nil
	}

	return tag, nil
}

func (s *Local) StoreTaskSucceedTag(taskName, tag string) error {
	if taskName == "" {
		return fmt.Errorf("task name can't be empty")
	}
	if tag == "" {
		return fmt.Errorf("tag can't be empty")
	}
	path := filepath.Join(s.path, taskName)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}

	filePath := filepath.Join(path, fileLastProcessedTag)

	return os.WriteFile(filePath, []byte(tag+"\n"), 0o644)
}
