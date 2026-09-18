package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The example config in the README used to nest commands and env under repo,
// where they do not belong, so copy-pasting it failed with
// "'repo' has invalid keys: commands, env".
func TestReadmeExampleLoads(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	require.NoError(t, err)

	block := regexp.MustCompile("(?s)```yaml\n(# trx\\.yaml\n.*?)```").FindSubmatch(readme)
	require.NotNil(t, block, "the README no longer contains the example config")

	path := filepath.Join(t.TempDir(), "trx.yaml")
	require.NoError(t, os.WriteFile(path, block[1], 0o644))

	cfg := &Config{}
	// Only the decoding: the example refers to key files that do not exist.
	require.NoError(t, loadConfig(path, nil, cfg, func() error { return nil }))

	require.NotEmpty(t, cfg.Commands)
	require.NotEmpty(t, cfg.Quorums)
	require.Equal(t, "production", cfg.Env[strings.ToLower("WERF_ENV")])
}
