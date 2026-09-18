package quorum

import (
	"fmt"
	"log"

	"github.com/go-git/go-git/v5"
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

func CheckQuorums(quorums []config.Quorum, repo *git.Repository, tag string) error {
	var g errgroup.Group
	for _, q := range quorums {
		g.Go(func() error {
			log.Printf("Verifying quorum %s\n", q.DisplayName())
			keys, err := q.AllGPGKeys()
			if err != nil {
				return &Error{QuorumName: q.DisplayName(), Err: fmt.Errorf("error reading GPG keys: %w", err)}
			}
			err = trdlGit.VerifyTagSignatures(repo, trdlGit.VerifyTagSignaturesRequest{
				Tag:          tag,
				NumberOfKeys: q.MinNumberOfKeys,
				GPGKeys:      keys,
			})
			if err != nil {
				return &Error{QuorumName: q.DisplayName(), Err: err}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
