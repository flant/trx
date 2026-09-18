package storage

import (
	"errors"
	"fmt"
	"log"
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

func NewLocalStorage(repoUrl string) (*Local, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("unable to determine the current user: %w", err)
	}

	storageDir := filepath.Join(usr.HomeDir, ".trx", "storage")
	s := &Local{path: filepath.Join(storageDir, git.RepoDirName(repoUrl))}

	if err := s.migrateLegacyState(filepath.Join(storageDir, git.RepoNameFromUrl(repoUrl))); err != nil {
		return nil, err
	}

	return s, nil
}

// migrateLegacyState copies the state written by an older version, which keyed
// the directory by the URL basename alone, to its URL-keyed location. The old
// directory is left in place: it may be shared with a repository of the same
// name, which would otherwise lose its last processed tag and redeploy.
func (s *Local) migrateLegacyState(legacyPath string) error {
	if legacyPath == s.path {
		return nil
	}
	if _, err := os.Stat(filepath.Join(s.path, fileLastProcessedCommit)); err == nil {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(legacyPath, fileLastProcessedCommit))
	if err != nil {
		return nil
	}

	if err := os.MkdirAll(s.path, 0o755); err != nil {
		return err
	}
	// Logged because the old directory may have been shared with a
	// repository of the same name, in which case the tag copied over is
	// whichever of the two ran last.
	log.Printf("Copying state %s to %s\n", legacyPath, s.path)
	return os.WriteFile(filepath.Join(s.path, fileLastProcessedCommit), data, 0o644)
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
