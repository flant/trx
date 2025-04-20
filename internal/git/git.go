package git

import (
	"fmt"
	"log"
	"net/url"
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

func RepoNameFromUrl(repoUrl string) string {
	repoUrl = strings.TrimSuffix(repoUrl, ".git")

	// SSH case: git@github.com:user/repo
	if strings.HasPrefix(repoUrl, "git@") {
		// git@github.com:user/repo → github.com/user/repo
		parts := strings.SplitN(repoUrl, ":", 2)
		host := strings.TrimPrefix(parts[0], "git@")
		path := parts[1]
		return toDottedName(host + "/" + path)
	}

	// HTTPS case: https://github.com/user/repo
	if u, err := url.Parse(repoUrl); err == nil {
		return toDottedName(u.Host + u.Path)
	}

	// fallback
	return toDottedName(repoUrl)
}

func toDottedName(s string) string {
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimSuffix(s, "/")
	parts := strings.Split(s, "/")
	return strings.Join(parts, ".")
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

		if currentVer.Equal(initialVer) && last == "" {
			log.Println("WARN current tag matches initial tag. Skipping as not newer.")
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
