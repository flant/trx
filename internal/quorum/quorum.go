package quorum

import (
	"errors"
	"fmt"
	"log"
	"os"

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
				r.HookExecutor.RunOnQuorumFailedHook(qErr.QuorumName)
			}
			return fmt.Errorf("quorum error: %w", qErr.Err)
		} else {
			return fmt.Errorf("quorum error: %w", err)
		}
	}
	return nil
}

func checkQuorums(quorums []config.Quorum, repo *git.Repository, tag string) error {
	var g errgroup.Group
	for _, q := range quorums {
		g.Go(func() error {
			log.Printf("Verifying quorum %s\n", *q.Name)
			keys, err := parseGPGKeys(q.GPGKeys, q.GPGKeyFilesPaths)
			if err != nil {
				return &Error{QuorumName: *q.Name, Err: fmt.Errorf("quorum `%s` error reading GPG keys: %w", *q.Name, err)}
			}
			err = trdlGit.VerifyTagSignatures(repo, trdlGit.VerifyTagSignaturesRequest{
				Tag:          tag,
				NumberOfKeys: q.MinNumberOfKeys,
				GPGKeys:      keys,
			})
			if err != nil {
				return &Error{QuorumName: *q.Name, Err: err}
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
