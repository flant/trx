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
	file_last_published = "last_published_commit"
)

type Local struct {
	path string
}

func NewLocalStorage(repoUrl string) *Local {
	usr, _ := user.Current()
	repoName := git.RepoNameFromUrl(repoUrl)
	return &Local{
		path: filepath.Join(usr.HomeDir, ".quorum-runner", "storage", repoName),
	}
}

func (s *Local) CheckLastSuccessedTag() (string, error) {
	filePath := filepath.Join(s.path, file_last_published)

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

func (s *Local) StoreSuccessedTag(commit string) error {
	if commit == "" {
		return fmt.Errorf("tag can't be empty")
	}

	if err := os.MkdirAll(s.path, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(s.path, file_last_published)

	return os.WriteFile(filePath, []byte(commit+"\n"), 0644)
}
