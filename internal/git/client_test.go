package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoDirNameFromUrl_uniquePerUrl(t *testing.T) {
	a := RepoDirNameFromUrl("git@github.com:org-a/infra.git")
	b := RepoDirNameFromUrl("git@github.com:org-b/infra.git")
	https := RepoDirNameFromUrl("https://github.com/org-a/infra.git")

	assert.NotEqual(t, a, b)
	assert.NotEqual(t, a, https)
	assert.Equal(t, a, RepoDirNameFromUrl("git@github.com:org-a/infra.git"))
	assert.Contains(t, a, "infra-")
	assert.NotContains(t, a, string(filepath.Separator))
}

func initLegacyClone(t *testing.T, baseDir, name, remoteUrl string) {
	t.Helper()
	repo, err := git.PlainInit(filepath.Join(baseDir, name), false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: git.DefaultRemoteName, URLs: []string{remoteUrl}})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "storage", name), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "storage", name, "last_processed_commit"), []byte("v1.0.0\n"), 0o644))
}

func TestMigrateLegacyDirs(t *testing.T) {
	url := "git@github.com:org-a/infra.git"

	t.Run("migrates when origin matches", func(t *testing.T) {
		baseDir := t.TempDir()
		initLegacyClone(t, baseDir, "infra", url)

		require.NoError(t, migrateLegacyDirs(baseDir, url))

		newName := RepoDirNameFromUrl(url)
		assert.NoDirExists(t, filepath.Join(baseDir, "infra"))
		assert.DirExists(t, filepath.Join(baseDir, newName))
		data, err := os.ReadFile(filepath.Join(baseDir, "storage", newName, "last_processed_commit"))
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0\n", string(data))
	})

	t.Run("keeps legacy dirs of another repo", func(t *testing.T) {
		baseDir := t.TempDir()
		initLegacyClone(t, baseDir, "infra", "git@github.com:org-b/infra.git")

		require.NoError(t, migrateLegacyDirs(baseDir, url))

		assert.DirExists(t, filepath.Join(baseDir, "infra"))
		assert.NoDirExists(t, filepath.Join(baseDir, RepoDirNameFromUrl(url)))
	})

	t.Run("no legacy clone is not an error", func(t *testing.T) {
		assert.NoError(t, migrateLegacyDirs(t.TempDir(), url))
	})
}

func TestCheckRemoteUrl(t *testing.T) {
	baseDir := t.TempDir()
	url := "git@github.com:org-a/infra.git"
	initLegacyClone(t, baseDir, "infra", url)

	repo, err := git.PlainOpen(filepath.Join(baseDir, "infra"))
	require.NoError(t, err)

	assert.NoError(t, checkRemoteUrl(repo, url))
	assert.Error(t, checkRemoteUrl(repo, "git@github.com:org-b/infra.git"))
}
