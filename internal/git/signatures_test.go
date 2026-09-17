package git

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/require"
)

// Regression test for "openpgp: unsupported feature: public key type: 22".
// Tag signatures made with EdDSA (Ed25519) keys must be verifiable, both alone
// and in a quorum mixed with RSA keys.
func TestVerifyTagSignatures_keyAlgorithms(t *testing.T) {
	eddsa := newTestEntity(t, "eddsa", packet.PubKeyAlgoEdDSA)
	rsa := newTestEntity(t, "rsa", packet.PubKeyAlgoRSA)
	stranger := newTestEntity(t, "stranger", packet.PubKeyAlgoEdDSA)

	tcs := []struct {
		name      string
		signer    *openpgp.Entity
		keys      []*openpgp.Entity
		wantError string
	}{
		{name: "eddsa signature, eddsa key", signer: eddsa, keys: []*openpgp.Entity{eddsa}},
		{name: "eddsa signature, mixed rsa+eddsa keys", signer: eddsa, keys: []*openpgp.Entity{rsa, eddsa}},
		{name: "rsa signature, mixed eddsa+rsa keys", signer: rsa, keys: []*openpgp.Entity{eddsa, rsa}},
		{name: "eddsa signature, untrusted key", signer: stranger, keys: []*openpgp.Entity{eddsa, rsa}, wantError: "not enough verified PGP signatures"},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			repo, tag := newRepoWithSignedTag(t, tc.signer)

			var keys []string
			for _, e := range tc.keys {
				keys = append(keys, armoredPublicKey(t, e))
			}

			err := VerifyTagSignatures(repo, VerifyTagSignaturesRequest{Tag: tag, NumberOfKeys: 1, GPGKeys: keys})
			if tc.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantError)
		})
	}
}

func newTestEntity(t *testing.T, name string, algo packet.PublicKeyAlgorithm) *openpgp.Entity {
	t.Helper()
	cfg := &packet.Config{Algorithm: algo}
	if algo == packet.PubKeyAlgoRSA {
		cfg.RSABits = 2048
	}
	e, err := openpgp.NewEntity(name, "", name+"@example.com", cfg)
	require.NoError(t, err)
	require.Equal(t, algo, e.PrimaryKey.PubKeyAlgo)
	return e
}

func armoredPublicKey(t *testing.T, e *openpgp.Entity) string {
	t.Helper()
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	require.NoError(t, err)
	require.NoError(t, e.Serialize(w))
	require.NoError(t, w.Close())
	return buf.String()
}

// newRepoWithSignedTag builds an in-memory repository with one commit and an
// annotated tag whose tag object is PGP-signed by signer.
func newRepoWithSignedTag(t *testing.T, signer *openpgp.Entity) (*git.Repository, string) {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	f, err := wt.Filesystem.Create("README")
	require.NoError(t, err)
	_, err = f.Write([]byte("test\n"))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	_, err = wt.Add("README")
	require.NoError(t, err)

	sig := &object.Signature{Name: "tester", Email: "tester@example.com", When: time.Now()}
	commit, err := wt.Commit("init", &git.CommitOptions{Author: sig, Committer: sig})
	require.NoError(t, err)

	const tagName = "v1.0.0"
	tag := &object.Tag{
		Name:       tagName,
		Tagger:     *sig,
		Message:    "release\n",
		TargetType: plumbing.CommitObject,
		Target:     commit,
	}

	payload := &plumbing.MemoryObject{}
	require.NoError(t, tag.EncodeWithoutSignature(payload))
	payloadReader, err := payload.Reader()
	require.NoError(t, err)

	var signature bytes.Buffer
	require.NoError(t, openpgp.ArmoredDetachSign(&signature, signer, payloadReader, nil))
	tag.PGPSignature = signature.String()

	encoded := repo.Storer.NewEncodedObject()
	require.NoError(t, tag.Encode(encoded))
	hash, err := repo.Storer.SetEncodedObject(encoded)
	require.NoError(t, err)
	require.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewTagReferenceName(tagName), hash)))

	// Sanity check: the stored object round-trips with its signature.
	stored, err := repo.TagObject(hash)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(stored.PGPSignature, "-----BEGIN PGP SIGNATURE-----"))

	return repo, tagName
}
