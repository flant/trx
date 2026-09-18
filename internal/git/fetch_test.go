package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

// Recovery from a bad newest tag is "delete it upstream", which only works if
// the fetch prunes. The signature note ref lives under refs/tags/ as well
// (refs/tags/latest-signature), so pruning must not take it with it.
func TestTagFetchOptions_prunesOnlyDeletedTags(t *testing.T) {
	upstreamDir := t.TempDir()
	upstream, err := git.PlainInit(upstreamDir, false)
	require.NoError(t, err)
	head := commitFile(t, upstream, upstreamDir, "a.txt")

	tagger := &object.Signature{Name: "trx", Email: "trx@example.com", When: time.Now()}
	_, err = upstream.CreateTag("v1.0.0", head, &git.CreateTagOptions{Message: "v1.0.0", Tagger: tagger})
	require.NoError(t, err)
	_, err = upstream.CreateTag("v2.0.0", head, nil)
	require.NoError(t, err)
	_, err = upstream.CreateTag("latest-signature", head, nil)
	require.NoError(t, err)

	repo, err := git.PlainClone(t.TempDir(), false, &git.CloneOptions{URL: upstreamDir})
	require.NoError(t, err)

	fetch := func() {
		t.Helper()
		if err := repo.Fetch(tagFetchOptions()); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			require.NoError(t, err)
		}
	}

	fetch()
	for _, tag := range []string{"v1.0.0", "v2.0.0", "latest-signature"} {
		_, err := repo.Tag(tag)
		require.NoErrorf(t, err, "tag %s must survive the fetch", tag)
	}

	require.NoError(t, upstream.DeleteTag("v2.0.0"))
	fetch()

	_, err = repo.Tag("v2.0.0")
	require.ErrorIs(t, err, git.ErrTagNotFound)
	_, err = repo.Tag("v1.0.0")
	require.NoError(t, err)
	_, err = repo.Tag("latest-signature")
	require.NoError(t, err)
}

// A directory left behind by an interrupted clone used to fail PlainOpen on
// every later run, with no way out but removing it by hand.
func TestOpenExistingClone_removesBrokenClone(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "infra-0123456789ab")
	require.NoError(t, os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755))

	repo, err := openExistingClone(repoPath, "git@github.com:flant/trx.git")
	require.NoError(t, err)
	require.Nil(t, repo, "a broken clone must not be reused")
	require.NoDirExists(t, repoPath)
}

// A clone of another repository is never reused, whatever its state.
func TestOpenExistingClone_refusesAForeignClone(t *testing.T) {
	repoPath := t.TempDir()
	repo, err := git.PlainInit(repoPath, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{"git@github.com:org-b/infra.git"},
	})
	require.NoError(t, err)

	_, err = openExistingClone(repoPath, "git@github.com:org-a/infra.git")
	require.ErrorContains(t, err, "has origin git@github.com:org-b/infra.git")
	require.DirExists(t, repoPath)
}

// The clone goes to a temporary directory and is renamed into place, so an
// interrupted clone leaves nothing behind for the next run to trip over.
func TestCloneRepo_movesTheCloneIntoPlace(t *testing.T) {
	upstreamDir := t.TempDir()
	upstream, err := git.PlainInit(upstreamDir, false)
	require.NoError(t, err)
	commitFile(t, upstream, upstreamDir, "a.txt")

	trxDir := filepath.Join(t.TempDir(), ".trx")
	repoPath := filepath.Join(trxDir, "infra-0123456789ab")

	repo, err := cloneRepo(context.Background(), &RepoConfig{Url: upstreamDir}, trxDir, repoPath)
	require.NoError(t, err)
	require.NotNil(t, repo)

	require.FileExists(t, filepath.Join(repoPath, "a.txt"))

	entries, err := os.ReadDir(trxDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "the temporary clone directory was left behind")
	require.Equal(t, filepath.Base(repoPath), entries[0].Name())

	url, err := originUrl(repo)
	require.NoError(t, err)
	require.Equal(t, upstreamDir, url)
}
