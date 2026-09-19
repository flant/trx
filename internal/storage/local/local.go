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
	return s.read(fileLastProcessedCommit)
}

func (s *Local) StoreSucceedTag(commit string) error {
	return s.write(fileLastProcessedCommit, commit)
}

func (s *Local) read(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.path, name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("error read from local storage: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// write replaces a state file atomically: a crash in the middle of a plain
// rewrite used to leave a truncated tag such as "v1.", which then failed every
// later run with "invalid last processed tag" until someone removed the file.
func (s *Local) write(name, value string) error {
	if value == "" {
		return fmt.Errorf("tag can't be empty")
	}

	if err := os.MkdirAll(s.path, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(s.path, name+".*")
	if err != nil {
		return fmt.Errorf("error write to local storage: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(value + "\n"); err != nil {
		tmp.Close()
		return fmt.Errorf("error write to local storage: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("error write to local storage: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("error write to local storage: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("error write to local storage: %w", err)
	}

	if err := os.Rename(tmp.Name(), filepath.Join(s.path, name)); err != nil {
		return fmt.Errorf("error write to local storage: %w", err)
	}

	// The rename itself is metadata of the directory, and is only durable
	// once the directory is synced.
	dir, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("error write to local storage: %w", err)
	}
	defer dir.Close()
	return dir.Sync()
}
