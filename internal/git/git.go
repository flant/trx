package git

import (
	"crypto/sha256"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/hashicorp/go-hclog"
	trdlGit "github.com/werf/trdl/server/pkg/git"
)

type VerifyTagSignaturesRequest struct {
	Tag          string
	NumberOfKeys int
	GPGKeys      []string
}

func VerifyTagSignatures(repo *git.Repository, r VerifyTagSignaturesRequest) error {
	log.Printf("Start verifyng signatures for tag %s\n", r.Tag)
	err := trdlGit.VerifyTagSignatures(repo, r.Tag, r.GPGKeys, r.NumberOfKeys, logger())
	if err != nil {
		return fmt.Errorf("unable to verify tag signatures: %w", err)
	}
	return nil
}

func RepoNameFromUrl(url string) string {
	return strings.TrimSuffix(path.Base(url), ".git")
}

// RepoDirName is the directory name a repository is cached and kept state
// under. The URL hash keeps repositories that share a basename, such as
// org-a/infra.git and org-b/infra.git, in separate clones and separate state.
func RepoDirName(url string) string {
	sum := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%s-%x", RepoNameFromUrl(url), sum[:6])
}

func IsNewerVersion(current, last, initial string) (bool, error) {
	currentVer, err := semver.NewVersion(current)
	if err != nil {
		return false, fmt.Errorf("invalid current tag: %w", err)
	}

	if initial != "" {
		initialVer, err := semver.NewVersion(initial)
		if err != nil {
			return false, fmt.Errorf("invalid initial tag: %w", err)
		}

		if currentVer.LessThanEqual(initialVer) {
			log.Printf("WARN current tag %s is less than or equal to initial tag %s", current, initial)
			return false, nil
		}
	}

	if last == "" {
		log.Println("WARN last processed tag is unknown. Processing without checking newer version")
		return true, nil
	}

	lastVer, err := semver.NewVersion(last)
	if err != nil {
		return false, fmt.Errorf("invalid last processed tag: %w", err)
	}

	return currentVer.GreaterThan(lastVer), nil
}

func logger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:  "trdl-lite",
		Level: hclog.LevelFromString("error"),
	})
}
