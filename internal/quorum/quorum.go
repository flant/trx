package quorum

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"golang.org/x/crypto/openpgp"
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

			keys, distinct, err := dedupeGPGKeys(keys)
			if err != nil {
				return &Error{QuorumName: name, Err: err}
			}
			if distinct < q.MinNumberOfKeys {
				return &Error{QuorumName: name, Err: fmt.Errorf(
					"number of distinct GPG keys is less than minNumberOfKeys. distinct: %d, minimum number: %d",
					distinct, q.MinNumberOfKeys)}
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

// dedupeGPGKeys drops key entries that hold no primary key fingerprint not seen
// before, so that the same key listed twice cannot satisfy a quorum of two. It
// returns the deduplicated entries and the number of distinct fingerprints.
func dedupeGPGKeys(keys []string) ([]string, int, error) {
	seen := make(map[string]bool)
	var res []string
	for _, key := range keys {
		fingerprints, err := keyFingerprints(key)
		if err != nil {
			return nil, 0, err
		}
		var isNew bool
		for _, fp := range fingerprints {
			if !seen[fp] {
				seen[fp] = true
				isNew = true
			}
		}
		if isNew {
			res = append(res, key)
		} else {
			log.Printf("WARNING duplicate GPG key %s is ignored\n", strings.Join(fingerprints, ", "))
		}
	}
	return res, len(seen), nil
}

func keyFingerprints(key string) ([]string, error) {
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(key))
	if err != nil {
		return nil, fmt.Errorf("unable to parse GPG key: %w", err)
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("no public key found in GPG key entry")
	}
	var fingerprints []string
	for _, e := range entities {
		if e.PrimaryKey == nil {
			return nil, fmt.Errorf("no public key found in GPG key entry")
		}
		fingerprints = append(fingerprints, hex.EncodeToString(e.PrimaryKey.Fingerprint[:]))
	}
	return fingerprints, nil
}
