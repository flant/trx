package git

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"trx/internal/command"
	"trx/internal/config"

	"log"

	"github.com/Masterminds/semver/v3"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hashicorp/go-hclog"
	trdlGit "github.com/werf/trdl/server/pkg/git"
)

type GitRepo struct {
	Url  string
	Auth *Auth
}

type Auth struct {
	AuthMetod transport.AuthMethod
}

func NewGitRepo(config config.GitRepo) (*GitRepo, error) {
	if config.Url == "" {
		return nil, fmt.Errorf("git url not specified")
	}

	if config.Auth.BasicAuth != nil {
		auth, err := NewBasicAuth(config.Auth.BasicAuth.Username, config.Auth.BasicAuth.Password)
		if err != nil {
			return nil, err
		}
		return &GitRepo{
			Url:  config.Url,
			Auth: auth,
		}, nil
	}

	auth, err := NewSshAuth(config.Auth.SshKeyPath, config.Auth.SshKeyPassword)
	if err != nil {
		return nil, err
	}
	return &GitRepo{
		Url:  config.Url,
		Auth: auth,
	}, nil
}

func NewSshAuth(key, password string) (*Auth, error) {
	if key == "" {
		return nil, nil
	}
	sshKey, _ := os.ReadFile(key)
	publicKey, err := ssh.NewPublicKeys("git", []byte(sshKey), password)
	if err != nil {
		return nil, fmt.Errorf("unable to get ssh public key: %w", err)
	}
	return &Auth{
		AuthMetod: publicKey,
	}, nil
}

func NewBasicAuth(username, password string) (*Auth, error) {
	return &Auth{
		AuthMetod: &http.BasicAuth{
			Username: username,
			Password: password,
		},
	}, nil
}

func (r *GitRepo) Open() (*git.Repository, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	repoName := RepoNameFromUrl(r.Url)
	repoPath := filepath.Join(usr.HomeDir, ".quorum-runner", repoName)

	var repo *git.Repository
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		cloneOptions := &git.CloneOptions{URL: r.Url}
		if r.Auth != nil {
			cloneOptions.Auth = r.Auth.AuthMetod
		}

		log.Printf("Cloning %s into %s\n", r.Url, repoPath)
		repo, err = git.PlainClone(repoPath, false, cloneOptions)
		if err != nil {
			return nil, fmt.Errorf("unable to clone repo: %w", err)
		}
		log.Println("Cloning done")
	} else {
		repo, err = git.PlainOpen(repoPath)
		if err != nil {
			return nil, fmt.Errorf("unable to open repo: %w", err)
		}
	}

	command.WorkDir = repoPath
	log.Println("Fetching tags")
	fetchOptions := &git.FetchOptions{
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec("refs/tags/*:refs/tags/*"),
		},
	}
	if r.Auth != nil {
		fetchOptions.Auth = r.Auth.AuthMetod
	}
	err = repo.Fetch(fetchOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, fmt.Errorf("unable to fetch tags: %w", err)
	}

	return repo, nil
}

type VerifyTagSignaturesRequest struct {
	Tag          string
	NumberOfKeys int
	GPGKeys      []string
}

func Verify(repo *git.Repository, r VerifyTagSignaturesRequest) error {
	log.Printf("Start verifyng signatures for tag %s\n", r.Tag)
	err := trdlGit.VerifyTagSignatures(repo, r.Tag, r.GPGKeys, r.NumberOfKeys, logger())
	if err != nil {
		return fmt.Errorf("unable to verify tag signatures: %w", err)
	}
	return nil
}

func GetLastSemverTag(repo *git.Repository) (string, string, error) {
	tagRefs, err := repo.Tags()
	if err != nil {
		return "", "", err
	}

	var versions []*semver.Version
	var tagMap = make(map[string]plumbing.ReferenceName)

	err = tagRefs.ForEach(func(ref *plumbing.Reference) error {
		tagName := ref.Name().Short()
		v, err := semver.NewVersion(tagName)
		if err == nil {
			versions = append(versions, v)
			tagMap[v.Original()] = ref.Name()
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}

	if len(versions) == 0 {
		return "", "", fmt.Errorf("no semantic version tags found")
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))
	lastTag := versions[0].Original()
	refName := tagMap[lastTag]

	ref, err := repo.Reference(refName, true)
	if err != nil {
		return "", "", err
	}

	hash := ref.Hash()

	return lastTag, hash.String(), nil
}

func checkout(repo *git.Repository, tagName string) error {
	tagRef, err := repo.Tag(tagName)
	if err != nil {
		return fmt.Errorf("tag not found: %w", err)
	}
	tagHash := tagRef.Hash()
	tagObj, err := repo.Object(plumbing.TagObject, tagHash)
	if err == nil {
		annotatedTag, ok := tagObj.(*object.Tag)
		if ok {
			tagHash = annotatedTag.Target
		}
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("unable to get worktree: %w", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:  tagHash,
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("checkout error: %w", err)
	}
	return nil
}

func logger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:  "trdl-lite",
		Level: hclog.LevelFromString("error"),
	})
}

type TargetGitObject struct {
	Repository *git.Repository
	Tag        string
	Commit     string
}

func GetTargetGitObject(cfg config.GitRepo) (*TargetGitObject, error) {
	repo, err := NewGitRepo(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize git client error: %w", err)
	}

	g, err := repo.Open()
	if err != nil {
		return nil, fmt.Errorf("open git repo error: %w", err)
	}
	tag, commit, err := GetLastSemverTag(g)
	if err != nil {
		return nil, err
	}
	return &TargetGitObject{
		Repository: g,
		Tag:        tag,
		Commit:     commit,
	}, nil
}

func PerformCheckout(repo *git.Repository, tag string) error {
	log.Printf("Got last tag %s. Perform checkout\n", tag)
	return checkout(repo, tag)
}

func RepoNameFromUrl(url string) string {
	return strings.TrimSuffix(path.Base(url), ".git")
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
