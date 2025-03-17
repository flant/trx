package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"trx/internal/lock"
)

const (
	lockFileName = ".lock"
)

type LockFileInfo struct {
	User      string `json:"user"`
	CreatedAt string `json:"created_at"`
}

func (s *Local) CheckLock() error {
	filePath := lockfilePath(s.path)

	_, err := os.Stat(filePath)
	if err == nil {
		lockInfo, readErr := readLockFile(filePath)
		if readErr != nil {
			return fmt.Errorf("error reading lock file: %w", readErr)
		}
		return &lock.ErrLocked{User: lockInfo.User, CreatedAt: lockInfo.CreatedAt}
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("error checking lock file: %w", err)
}

func (s *Local) StoreLockInfo() error {
	filePath := lockfilePath(s.path)
	f, err := createLockFile(filePath)
	if err != nil {
		return fmt.Errorf("error creating lock file: %w", err)
	}
	s.lockFile = f
	return nil
}

func (s *Local) RemoveLockInfo() error {
	if s.lockFile == nil {
		return nil
	}
	s.lockFile.Close()
	return os.Remove(s.lockFile.Name())
}

func (s *Local) ForceRemoveLockInfo() error {
	f, err := tryGetLockFile(s.path)
	if err != nil {
		return err
	}
	f.Close()
	return os.Remove(f.Name())
}

func lockfilePath(s string) string {
	return filepath.Join(s, lockFileName)
}

func createLockFile(path string) (*os.File, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	lockInfo := LockFileInfo{
		User:      usr.Username,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(lockInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lock info: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create lock file directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	_, writeErr := file.Write(data)
	if writeErr != nil {
		file.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to write lock file: %w", writeErr)
	}

	return file, nil
}

func readLockFile(path string) (*LockFileInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lockInfo LockFileInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lock file: %w", err)
	}

	return &lockInfo, nil
}

func tryGetLockFile(path string) (*os.File, error) {
	file, err := os.Open(lockfilePath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	return file, nil
}
