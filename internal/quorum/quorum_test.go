package quorum

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/require"

	"trx/internal/config"
)

// name is optional in the config and validation tolerates its absence, but
// verification dereferenced it and panicked after the checkout, before any hook
// could report anything.
func TestCheckQuorums_unnamedQuorumDoesNotPanic(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	err = CheckQuorums([]config.Quorum{{MinNumberOfKeys: 1, GPGKeys: []string{"not a key"}}}, repo, "v1.0.0")
	require.Error(t, err)
	require.ErrorContains(t, err, "<unnamed>")
}
