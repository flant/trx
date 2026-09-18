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

// A tag that failed must be remembered, so the next run skips it instead of
// failing identically and firing its hook again, and must be forgotten as soon
// as a tag succeeds.
func TestFailedTag(t *testing.T) {
	s := &Local{path: filepath.Join(t.TempDir(), "infra-0123456789ab")}

	tag, err := s.CheckLastFailedTag()
	require.NoError(t, err)
	require.Empty(t, tag)

	require.NoError(t, s.StoreFailedTag("v1.2.3"))
	tag, err = s.CheckLastFailedTag()
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", tag)

	require.NoError(t, s.StoreSucceedTag("v1.2.4"))
	tag, err = s.CheckLastFailedTag()
	require.NoError(t, err)
	require.Empty(t, tag)
}

func TestMigrateLegacyState_noLegacyDir(t *testing.T) {
	dir := t.TempDir()
	s := &Local{path: filepath.Join(dir, "infra-0123456789ab")}

	require.NoError(t, s.migrateLegacyState(filepath.Join(dir, "infra")))

	tag, err := s.CheckLastSucceedTag()
	require.NoError(t, err)
	require.Empty(t, tag)
}
