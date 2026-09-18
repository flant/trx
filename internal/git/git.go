package git

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/user"
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

// HomeDir returns the directory trx keeps its clones and state in. $HOME is
// preferred, because a static binary running under a uid that is missing from
// /etc/passwd (the usual case for runAsUser in Kubernetes) cannot look itself
// up, but the passwd entry is still used when $HOME is not set.
func HomeDir() (string, error) {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home, nil
	}
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	if usr.HomeDir == "" {
		return "", fmt.Errorf("unable to determine home directory: user %s has no home directory", usr.Username)
	}
	return usr.HomeDir, nil
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
	name := strings.Join(parts, ".")
	// The result is used as a single path element, it must not contain a
	// separator of its own.
	name = strings.ReplaceAll(name, string(os.PathSeparator), ".")
	return strings.ReplaceAll(name, "..", ".")
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
