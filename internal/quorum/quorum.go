package quorum

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/sync/errgroup"

	"trx/internal/config"
	trdlGit "trx/internal/git"
)

type Error struct {
	QuorumName string
	Err        error
}

func (e *Error) Error() string {
	return fmt.Sprintf("quorum `%s` error: %v", e.QuorumName, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

type HookExecutor interface {
	RunOnQuorumFailedHook(quorumName string) error
}

type CheckQuorumsRequest struct {
	Quorums      []config.Quorum
	Repo         *git.Repository
	Tag          string
	HookExecutor HookExecutor
}

func CheckQuorums(r *CheckQuorumsRequest) error {
	if err := checkQuorums(r.Quorums, r.Repo, r.Tag); err != nil {
		var qErr *Error
		if errors.As(err, &qErr) {
			if r.HookExecutor != nil {
				_ = r.HookExecutor.RunOnQuorumFailedHook(qErr.QuorumName)
			}
		}
		return fmt.Errorf("quorum error: %w", err)
	}
	return nil
}

func checkQuorums(quorums []config.Quorum, repo *git.Repository, tag string) error {
	var g errgroup.Group
	for i, q := range quorums {
		name := quorumName(q, i)
		g.Go(func() error {
			log.Printf("Verifying quorum %s\n", name)
			keys, err := parseGPGKeys(q.GPGKeys, q.GPGKeyFilesPaths)
			if err != nil {
				return &Error{QuorumName: name, Err: fmt.Errorf("error reading GPG keys: %w", err)}
			}

			keys, err = dedupeGPGKeys(keys)
			if err != nil {
				return &Error{QuorumName: name, Err: err}
			}
			if len(keys) < q.MinNumberOfKeys {
				return &Error{QuorumName: name, Err: fmt.Errorf(
					"number of distinct GPG keys is less than minNumberOfKeys. distinct: %d, minimum number: %d",
					len(keys), q.MinNumberOfKeys)}
			}

			err = trdlGit.VerifyTagSignatures(repo, trdlGit.VerifyTagSignaturesRequest{
				Tag:          tag,
				NumberOfKeys: q.MinNumberOfKeys,
				GPGKeys:      keys,
			})
			if err != nil {
				return &Error{QuorumName: name, Err: err}
			}
			return nil
		})
	}
	return g.Wait()
}

func quorumName(q config.Quorum, i int) string {
	if q.Name != nil && *q.Name != "" {
		return *q.Name
	}
	return fmt.Sprintf("#%d", i+1)
}

func parseGPGKeys(plain, files []string) ([]string, error) {
	var res []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("error read key file %s: %w", f, err)
		}
		res = append(res, string(data))
	}

	return append(res, plain...), nil
}

// dedupeGPGKeys splits every key entry into one entry per key and drops keys
// already seen, so that the same key listed twice cannot satisfy a quorum of
// two: verification counts entries and removes only the entry that matched.
// It returns one entry per distinct key.
func dedupeGPGKeys(keys []string) ([]string, error) {
	seen := make(map[string]bool)
	var res []string
	for _, key := range keys {
		entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(key))
		if err != nil {
			return nil, fmt.Errorf("unable to parse GPG key: %w", err)
		}
		if len(entities) == 0 {
			return nil, fmt.Errorf("no public key found in GPG key entry")
		}
		for _, entity := range entities {
			if entity.PrimaryKey == nil {
				return nil, fmt.Errorf("no public key found in GPG key entry")
			}
			fingerprint := hex.EncodeToString(entity.PrimaryKey.Fingerprint[:])
			if seen[fingerprint] {
				log.Printf("WARNING duplicate GPG key %s is ignored\n", fingerprint)
				continue
			}
			seen[fingerprint] = true

			armored, err := armorEntity(entity)
			if err != nil {
				return nil, fmt.Errorf("unable to read GPG key %s: %w", fingerprint, err)
			}
			res = append(res, armored)
		}
	}
	return res, nil
}

func armorEntity(entity *openpgp.Entity) (string, error) {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", err
	}
	if err := entity.Serialize(w); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
