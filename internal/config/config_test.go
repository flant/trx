package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/require"
)

// armoredEd25519PublicKey returns a freshly generated Ed25519 (EdDSA, OpenPGP
// algorithm 22) public key. Such keys used to be rejected with
// "openpgp: unsupported feature: public key type: 22".
func armoredEd25519PublicKey(t *testing.T) string {
	t.Helper()

	e, err := openpgp.NewEntity("ed25519", "", "ed25519@example.com", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	require.NoError(t, err)
	require.Equal(t, packet.PubKeyAlgoEdDSA, e.PrimaryKey.PubKeyAlgo)

	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	require.NoError(t, err)
	require.NoError(t, e.Serialize(w))
	require.NoError(t, w.Close())
	return buf.String()
}

func TestValidateQuorums_gpgKeys(t *testing.T) {
	name := "main"
	ed25519PublicKey := armoredEd25519PublicKey(t)
	otherPublicKey := armoredEd25519PublicKey(t)

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
		{
			name:   "two distinct keys",
			quorum: Quorum{Name: &name, MinNumberOfKeys: 2, GPGKeys: []string{ed25519PublicKey, otherPublicKey}},
		},
		{
			name:      "the same key listed twice inline",
			quorum:    Quorum{Name: &name, MinNumberOfKeys: 2, GPGKeys: []string{ed25519PublicKey, ed25519PublicKey}},
			wantError: `quorum "main" gpgKeys[1]: duplicates the GPG key`,
		},
		{
			name:      "the same key inline and from a file",
			quorum:    Quorum{Name: &name, MinNumberOfKeys: 2, GPGKeys: []string{ed25519PublicKey}, GPGKeyFilesPaths: []string{keyFile}},
			wantError: `quorum "main" gpgKeys[0]: duplicates the GPG key`,
		},
		{
			// The verifier drops the whole entry once one of its keys
			// matched, so such an entry can never count for two.
			name:      "two keys in one entry",
			quorum:    Quorum{Name: &name, MinNumberOfKeys: 2, GPGKeys: []string{ed25519PublicKey + otherPublicKey}},
			wantError: "number of GPG keys is less then number of minimum GPG keys",
		},
		{
			name:      "fewer keys than minNumberOfKeys",
			quorum:    Quorum{Name: &name, MinNumberOfKeys: 2, GPGKeys: []string{ed25519PublicKey}},
			wantError: "number of GPG keys is less then number of minimum GPG keys",
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
