package tasks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubStorage struct {
	last     string
	readErr  error
	storeErr error
	stored   map[string]string
}

func (s *stubStorage) CheckTaskLastSucceedTag(string) (string, error) {
	return s.last, s.readErr
}

func (s *stubStorage) StoreTaskSucceedTag(taskName, commit string) error {
	if s.storeErr != nil {
		return s.storeErr
	}
	if s.stored == nil {
		s.stored = map[string]string{}
	}
	s.stored[taskName] = commit
	return nil
}

type stubExecutor struct{ runs int }

func (e *stubExecutor) Exec([]string, map[string]string, map[string]string) error {
	e.runs++
	return nil
}

func runTasks(t *testing.T, storage *stubStorage) (*stubExecutor, error) {
	t.Helper()
	e := &stubExecutor{}
	executor := &TaskExecutor{executor: e, storage: storage}
	return e, executor.RunTasks([]Task{{Name: "deploy", Version: "v1.0.0", Commands: []string{"true"}}})
}

// A corrupt or unreadable state file must not look like "nothing to do".
func TestRunTasksStateReadFailure(t *testing.T) {
	exec, err := runTasks(t, &stubStorage{last: "not-a-version"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrStateReadFailed), err)
	assert.False(t, errors.Is(err, ErrNoNewVersion))
	assert.Equal(t, 0, exec.runs)
}

func TestRunTasksStateStoreFailure(t *testing.T) {
	exec, err := runTasks(t, &stubStorage{storeErr: errors.New("read-only file system")})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrStateStoreFailed), err)
	assert.Equal(t, 1, exec.runs, "the task did run, only the state was lost")
}

func TestRunTasksNoNewVersion(t *testing.T) {
	exec, err := runTasks(t, &stubStorage{last: "v1.0.0"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoNewVersion), err)
	assert.Equal(t, 0, exec.runs)
}

func TestRunTasksStoresTag(t *testing.T) {
	storage := &stubStorage{}
	exec, err := runTasks(t, storage)
	require.NoError(t, err)
	assert.Equal(t, 1, exec.runs)
	assert.Equal(t, "v1.0.0", storage.stored["deploy"])
}
