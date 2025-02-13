package quorum

import (
	"fmt"
	"log"
	"os"
	"trx/internal/config"
	trdlGit "trx/internal/git"

	"github.com/go-git/go-git/v5"
	"golang.org/x/sync/errgroup"
)

type QuorumError struct {
	QuorumName string
	Err        error
}

func (e *QuorumError) Error() string {
	return fmt.Sprintf("quorum `%s` error: %v", e.QuorumName, e.Err)
}

func (e *QuorumError) Unwrap() error {
	return e.Err
}

func CheckQuorums(quorums []config.Quorum, repo *git.Repository, tag string) error {
	var g errgroup.Group
	for _, q := range quorums {
		g.Go(func() error {
			log.Printf("Verifying quorum %s\n", *q.Name)
			keys, err := parseGPGKeys(q.GPGKeys, q.GPGKeyFilesPaths)
			if err != nil {
				return &QuorumError{QuorumName: *q.Name, Err: fmt.Errorf("quorum `%s` error reading GPG keys: %w", *q.Name, err)}
			}
			err = trdlGit.Verify(repo, trdlGit.VerifyTagSignaturesRequest{
				Tag:          tag,
				NumberOfKeys: q.MinNumberOfKeys,
				GPGKeys:      keys,
			})
			if err != nil {
				return &QuorumError{QuorumName: *q.Name, Err: err}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func parseGPGKeys(plain, files []string) ([]string, error) {
	res := []string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("error read key file %s: %w", f, err)
		}
		res = append(res, string(data))
	}

	return append(res, plain...), nil
}
