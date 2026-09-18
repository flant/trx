package quorum

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"trx/internal/config"
)

func TestDedupeGPGKeys(t *testing.T) {
	key, err := os.ReadFile("../../trx.asc")
	require.NoError(t, err)

	keys, distinct, err := dedupeGPGKeys([]string{string(key), string(key)})
	require.NoError(t, err)
	assert.Equal(t, 1, distinct, "the same key listed twice must not count as two")
	assert.Len(t, keys, 1)
}

func TestDedupeGPGKeysInvalid(t *testing.T) {
	_, _, err := dedupeGPGKeys([]string{"not a key"})
	assert.Error(t, err)
}

func TestQuorumNameWithoutName(t *testing.T) {
	assert.Equal(t, "#2", quorumName(config.Quorum{}, 1))
}
