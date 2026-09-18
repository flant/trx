package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// State written under the old basename-only directory must survive the move to
// the URL-keyed one, otherwise the next run sees no last processed tag and
// redeploys the current one.
func TestMigrateLegacyState(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "infra")
	require.NoError(t, os.MkdirAll(legacyPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyPath, fileLastProcessedCommit), []byte("v1.2.3\n"), 0o644))

	s := &Local{path: filepath.Join(dir, "infra-0123456789ab")}
	require.NoError(t, s.migrateLegacyState(legacyPath))

	tag, err := s.CheckLastSucceedTag()
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", tag)

	// The legacy directory stays: a repository of the same name may still
	// need it.
	require.FileExists(t, filepath.Join(legacyPath, fileLastProcessedCommit))

	// An already migrated state is never overwritten.
	require.NoError(t, s.StoreSucceedTag("v2.0.0"))
	require.NoError(t, s.migrateLegacyState(legacyPath))
	tag, err = s.CheckLastSucceedTag()
	require.NoError(t, err)
	require.Equal(t, "v2.0.0", tag)
}

func TestMigrateLegacyState_noLegacyDir(t *testing.T) {
	dir := t.TempDir()
	s := &Local{path: filepath.Join(dir, "infra-0123456789ab")}

	require.NoError(t, s.migrateLegacyState(filepath.Join(dir, "infra")))

	tag, err := s.CheckLastSucceedTag()
	require.NoError(t, err)
	require.Empty(t, tag)
}
