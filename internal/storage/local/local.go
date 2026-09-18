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

const TypeLocalStorage = "local"

const (
	fileLastProcessedCommit = "last_processed_commit"
)

type Local struct {
	path string
}

func NewLocalStorage(repoUrl string) *Local {
	usr, _ := user.Current()
	return &Local{
		path: filepath.Join(usr.HomeDir, ".trx", "storage", git.RepoDirNameFromUrl(repoUrl)),
	}
}

func (s *Local) CheckLastSucceedTag() (string, error) {
	filePath := filepath.Join(s.path, fileLastProcessedCommit)

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

func (s *Local) StoreSucceedTag(commit string) error {
	if commit == "" {
		return fmt.Errorf("tag can't be empty")
	}

	if err := os.MkdirAll(s.path, 0o755); err != nil {
		return err
	}

	filePath := filepath.Join(s.path, fileLastProcessedCommit)

	return os.WriteFile(filePath, []byte(commit+"\n"), 0o644)
}
