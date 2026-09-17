package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Ed25519 (EdDSA, OpenPGP algorithm 22) public key. Previously rejected with
// "openpgp: unsupported feature: public key type: 22".
const ed25519PublicKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mDMEaia9LhYJKwYBBAHaRw8BAQdAw+PghBdifN4xQ3Nv99SVTn1DCGafIGUlG26o
Q4i4lc+0J1Zhc2lseSBNYXJtZXIgPHZhc2lseS5tYXJtZXJAZmxhbnQuY29tPoiQ
BBMWCAA4FiEEWhGJWKZFuSvkyqcPKqrWsAV/KOoFAmomvS4CGwMFCwkIBwIGFQoJ
CAsCBBYCAwECHgECF4AACgkQKqrWsAV/KOorowEA8D4chmaRPfojb3nKBsHykWve
JAHPCp92HaRZBTwtxYUBAIdPPOM9+o0hiA66HNWe2FyU1QL+vK1alEDXDAF3nNAL
uDMEaia9LhYJKwYBBAHaRw8BAQdACRh9NuRNrlsjbnedpZ197Bl0/lj4lu6Qj0hh
W+z8MVqIeAQYFggAIBYhBFoRiVimRbkr5MqnDyqq1rAFfyjqBQJqJr0uAhsgAAoJ
ECqq1rAFfyjqGgoA/1vb368vJ3HVN9IqWJWQUXMZRboU3Ci/zFWHUgRTa5XjAQCH
8AVT5sklvyscDwN8vwXUFbRlDUsGY9UtMDmya4FCCrg4BGomvS4SCisGAQQBl1UB
BQEBB0DzBG5WLGqsxx4Af7VRz6/u+7/R68cFyZqcFilvo5xiJAMBCAeIeAQYFggA
IBYhBFoRiVimRbkr5MqnDyqq1rAFfyjqBQJqJr0uAhsMAAoJECqq1rAFfyjqqmIB
AN+KBpx+d2jboZS/+4PI0BXum3p8gav2SsTisIahVmkAAP9RZB+anT8o16fvIZKx
UovcZ/dYqWhi9mH/YPIwbPZlDA==
=C7cg
-----END PGP PUBLIC KEY BLOCK-----
`

func TestValidateQuorums_gpgKeys(t *testing.T) {
	name := "main"

	keyFile := filepath.Join(t.TempDir(), "key.asc")
	require.NoError(t, os.WriteFile(keyFile, []byte(ed25519PublicKey), 0o600))

	brokenFile := filepath.Join(t.TempDir(), "broken.asc")
	require.NoError(t, os.WriteFile(brokenFile, []byte("not a key"), 0o600))

	tcs := []struct {
		name      string
		quorum    Quorum
		wantError string
	}{
		{
			name:   "ed25519 inline key",
			quorum: Quorum{Name: &name, MinNumberOfKeys: 1, GPGKeys: []string{ed25519PublicKey}},
		},
		{
			name:   "ed25519 key from file",
			quorum: Quorum{Name: &name, MinNumberOfKeys: 1, GPGKeyFilesPaths: []string{keyFile}},
		},
		{
			name:      "garbage inline key",
			quorum:    Quorum{Name: &name, MinNumberOfKeys: 1, GPGKeys: []string{ed25519PublicKey, "not a key"}},
			wantError: `quorum "main" gpgKeys[1]: invalid GPG public key`,
		},
		{
			name:      "garbage key file",
			quorum:    Quorum{Name: &name, MinNumberOfKeys: 1, GPGKeyFilesPaths: []string{brokenFile}},
			wantError: `quorum "main" gpgKeyPaths[0]`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateQuorums([]Quorum{tc.quorum})
			if tc.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantError)
		})
	}
}
