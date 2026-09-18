package git

import (
	"errors"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
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
