package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"trx/internal/config"
)

// An unreadable ssh key produced a confusing "unable to get ssh public key"
// instead of naming the file and the I/O error.
func TestNewRepoConfig_missingSshKeyNamesTheFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "id_ed25519")

	_, err := NewRepoConfig(config.GitRepo{
		Url:  "git@github.com:flant/trx.git",
		Auth: config.GitRepoAuth{SshKeyPath: missing},
	})

	require.ErrorContains(t, err, "unable to read the ssh key "+missing)
}
