package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"trx/internal/git"
)

const TypeLocalStorage = "local"

const (
	fileLastProcessedCommit = "last_processed_commit"
)

type Local struct {
	path string
}

func NewLocalStorage(repoUrl string) (*Local, error) {
	home, err := git.HomeDir()
	if err != nil {
		return nil, err
	}
	repoName := git.RepoNameFromUrl(repoUrl)
	return &Local{
		path: filepath.Join(home, ".trx", "storage", repoName),
	}, nil
}

func (s *Local) CheckTaskLastSucceedTag(taskName string) (string, error) {
	if taskName == "" {
		return "", fmt.Errorf("task name can't be empty")
	}
	path := filepath.Join(s.path, taskName)
	filePath := filepath.Join(path, fileLastProcessedCommit)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("error read from local storage: %w", err)
	}

	commit := strings.TrimSpace(string(data))
	if commit == "" {
		return "", nil
	}

	return commit, nil
}

func (s *Local) StoreTaskSucceedTag(taskName, commit string) error {
	if taskName == "" {
		return fmt.Errorf("task name can't be empty")
	}
	if commit == "" {
		return fmt.Errorf("tag can't be empty")
	}
	path := filepath.Join(s.path, taskName)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}

	filePath := filepath.Join(path, fileLastProcessedCommit)
	return writeFileAtomic(filePath, []byte(commit+"\n"))
}

// writeFileAtomic writes through a temporary file in the same directory and
// renames it into place, so that a crash mid-write cannot leave a truncated
// tag behind.
func writeFileAtomic(filePath string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(filePath), "."+filepath.Base(filePath)+".*")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	defer func() {
		f.Close()
		os.Remove(tmpPath)
	}()

	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, filePath)
}
