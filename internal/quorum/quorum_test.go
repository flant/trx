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

	keys, err := dedupeGPGKeys([]string{string(key), string(key)})
	require.NoError(t, err)
	assert.Len(t, keys, 1, "the same key listed twice must not count as two")

	// The re-armored key must still be a usable key ring.
	again, err := dedupeGPGKeys(keys)
	require.NoError(t, err)
	assert.Len(t, again, 1)
}

func TestDedupeGPGKeysInvalid(t *testing.T) {
	_, err := dedupeGPGKeys([]string{"not a key"})
	assert.Error(t, err)
}

func TestQuorumNameWithoutName(t *testing.T) {
	assert.Equal(t, "#2", quorumName(config.Quorum{}, 1))
}
