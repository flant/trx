package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

// Resolving the tag to deploy must not touch the worktree: the hooks that run
// before quorum verification (onQuorumFailure, onCommandSkipped) would
// otherwise execute with the content of an unverified tag as their working
// directory. Only Checkout, called after verification, may move the worktree.
func TestGetTargetGitObject_doesNotCheckout(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	tagged := commitFile(t, repo, dir, "tagged.txt")
	_, err = repo.CreateTag("v1.0.0", tagged, nil)
	require.NoError(t, err)
	head := commitFile(t, repo, dir, "untagged.txt")

	client := &GitClient{Repo: repo, RepoPath: dir}

	target, err := client.GetTargetGitObject()
	require.NoError(t, err)
	require.Equal(t, "v1.0.0", target.Tag)

	ref, err := repo.Head()
	require.NoError(t, err)
	require.Equal(t, head, ref.Hash())
	require.FileExists(t, filepath.Join(dir, "untagged.txt"))

	require.NoError(t, client.Checkout(target))
	require.NoFileExists(t, filepath.Join(dir, "untagged.txt"))
}

func commitFile(t *testing.T, repo *git.Repository, dir, name string) plumbing.Hash {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644))

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add(name)
	require.NoError(t, err)

	hash, err := wt.Commit("add "+name, &git.CommitOptions{
		Author: &object.Signature{Name: "trx", Email: "trx@example.com", When: time.Now()},
	})
	require.NoError(t, err)
	return hash
}

// Signed releases use annotated tags, whose own object hash is not a commit.
// {{ .RepoCommit }} used to be that object hash.
func TestGetLastSemverTag_annotatedTagResolvesToCommit(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	commit := commitFile(t, repo, dir, "a.txt")
	tagRef, err := repo.CreateTag("v1.0.0", commit, &git.CreateTagOptions{
		Message: "v1.0.0",
		Tagger:  &object.Signature{Name: "trx", Email: "trx@example.com", When: time.Now()},
	})
	require.NoError(t, err)
	require.NotEqual(t, commit, tagRef.Hash(), "the annotated tag must be an object of its own")

	client := &GitClient{Repo: repo, RepoPath: dir}
	tag, hash, err := client.GetLastSemverTag()
	require.NoError(t, err)
	require.Equal(t, "v1.0.0", tag)
	require.Equal(t, commit.String(), hash)
}
