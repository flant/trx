package git

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"trx/internal/config"
)

type GitClient struct {
	Repo *git.Repository
	// allowPrerelease lets pre-release tags be deployed, for a setup that
	// releases through -rc tags on purpose.
	allowPrerelease bool
	// RepoPath is the local clone: the working directory of the commands,
	// but only once the tag they come from has passed quorum verification.
	RepoPath string
}

func NewGitClient(ctx context.Context, cfg config.GitRepo) (*GitClient, error) {
	repoConf, err := NewRepoConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("new repo config error: %w", err)
	}

	repo, repoPath, err := openGitRepo(ctx, repoConf)
	if err != nil {
		return nil, fmt.Errorf("open git repo error: %w", err)
	}

	return &GitClient{
		Repo:            repo,
		RepoPath:        repoPath,
		allowPrerelease: cfg.AllowPrerelease,
	}, nil
}

// GetTargetGitObject resolves the tag to deploy without touching the worktree:
// nothing may check out an unverified tag, because the hooks that run before
// verification would execute with its content as their working directory.
func (g *GitClient) GetTargetGitObject() (*TargetGitObject, error) {
	tag, commit, err := g.GetLastSemverTag()
	if err != nil {
		return nil, err
	}
	return &TargetGitObject{Tag: tag, Commit: commit}, nil
}

type TargetGitObject struct {
	Tag    string
	Commit string
}

// commitHash resolves a tag to the commit it points at. An annotated tag is an
// object of its own, and its hash is not a commit: signed releases use
// annotated tags, so this is the normal case rather than the exception.
func (g *GitClient) commitHash(tag string) (plumbing.Hash, error) {
	tagRef, err := g.Repo.Tag(tag)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("tag not found: %w", err)
	}

	tagObj, err := g.Repo.TagObject(tagRef.Hash())
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return tagRef.Hash(), nil
	}
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("unable to read tag %s: %w", tag, err)
	}

	commit, err := tagObj.Commit()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("unable to resolve the commit of tag %s: %w", tag, err)
	}
	return commit.Hash, nil
}

func (g *GitClient) Checkout(o *TargetGitObject) error {
	log.Printf("Got last tag %s. Perform checkout\n", o.Tag)
	tagHash, err := g.commitHash(o.Tag)
	if err != nil {
		return err
	}

	worktree, err := g.Repo.Worktree()
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

	// A forced checkout leaves untracked files alone, so build artifacts of
	// the previous release would otherwise take part in the next deployment.
	if err := worktree.Clean(&git.CleanOptions{Dir: true}); err != nil {
		return fmt.Errorf("unable to clean the worktree: %w", err)
	}
	return nil
}

func (g *GitClient) GetLastSemverTag() (string, string, error) {
	tagRefs, err := g.Repo.Tags()
	if err != nil {
		return "", "", err
	}

	var versions []*semver.Version

	// Pre-releases are skipped unless they are asked for: semver sorts
	// v2.0.0-rc1 above v1.9.0, so a release candidate would otherwise become
	// the newest tag and be deployed as a release.
	err = tagRefs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		v, err := semver.NewVersion(name)
		if err != nil {
			return nil
		}
		if v.Prerelease() != "" && !g.allowPrerelease {
			log.Printf("Skipping pre-release tag %s\n", name)
			return nil
		}
		versions = append(versions, v)
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

	hash, err := g.commitHash(lastTag)
	if err != nil {
		return "", "", err
	}

	return lastTag, hash.String(), nil
}

// gitOperationTimeout bounds a clone and a fetch. The execution lock is already
// held while they run, so a remote that accepts the connection and then goes
// silent would otherwise block every later run as well.
const gitOperationTimeout = 15 * time.Minute

func openGitRepo(ctx context.Context, r *RepoConfig) (*git.Repository, string, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, "", err
	}

	trxDir := filepath.Join(usr.HomeDir, ".trx")
	repoPath := filepath.Join(trxDir, RepoDirName(r.Url))

	if err := migrateLegacyClone(filepath.Join(trxDir, RepoNameFromUrl(r.Url)), repoPath, r.Url); err != nil {
		return nil, "", err
	}

	repo, err := openExistingClone(repoPath, r.Url)
	if err != nil {
		return nil, "", err
	}
	if repo == nil {
		repo, err = cloneRepo(ctx, r, trxDir, repoPath)
		if err != nil {
			return nil, "", err
		}
	}

	log.Println("Fetching tags")
	fetchOptions := tagFetchOptions()
	if r.Auth != nil {
		fetchOptions.Auth = r.Auth.AuthMethod
	}

	fetchCtx, cancel := context.WithTimeout(ctx, gitOperationTimeout)
	defer cancel()
	err = repo.FetchContext(fetchCtx, fetchOptions)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, "", fmt.Errorf("unable to fetch tags: %w", err)
	}

	return repo, repoPath, nil
}

// openExistingClone returns the clone at repoPath, or nil when there is none to
// use. A directory left behind by an interrupted clone is removed instead of
// failing every run from now on, but a clone of a different repository is an
// error: nothing may fetch into, verify or run commands in it.
func openExistingClone(repoPath, url string) (*git.Repository, error) {
	if _, err := os.Stat(repoPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to stat the clone %s: %w", repoPath, err)
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		log.Printf("Removing the unusable clone %s (%s)\n", repoPath, err.Error())
		if err := os.RemoveAll(repoPath); err != nil {
			return nil, fmt.Errorf("unable to remove the unusable clone %s: %w", repoPath, err)
		}
		return nil, nil
	}

	clonedUrl, err := originUrl(repo)
	if err != nil {
		return nil, fmt.Errorf("unable to read origin of the existing clone %s: %w", repoPath, err)
	}
	if clonedUrl != url {
		return nil, fmt.Errorf("existing clone %s has origin %s, expected %s: remove the directory to re-clone", repoPath, clonedUrl, url)
	}

	return repo, nil
}

// cloneRepo clones into a temporary directory and renames it into place, so an
// interrupted clone leaves no half-written repository behind.
func cloneRepo(ctx context.Context, r *RepoConfig, trxDir, repoPath string) (*git.Repository, error) {
	if err := os.MkdirAll(trxDir, 0o755); err != nil {
		return nil, fmt.Errorf("unable to create %s: %w", trxDir, err)
	}

	tmpPath, err := os.MkdirTemp(trxDir, ".clone-*")
	if err != nil {
		return nil, fmt.Errorf("unable to create a temporary clone directory: %w", err)
	}
	defer os.RemoveAll(tmpPath)

	cloneOptions := &git.CloneOptions{URL: r.Url}
	if r.Auth != nil {
		cloneOptions.Auth = r.Auth.AuthMethod
	}

	log.Printf("Cloning %s into %s\n", r.Url, repoPath)
	cloneCtx, cancel := context.WithTimeout(ctx, gitOperationTimeout)
	defer cancel()
	if _, err := git.PlainCloneContext(cloneCtx, tmpPath, false, cloneOptions); err != nil {
		return nil, fmt.Errorf("unable to clone repo: %w", err)
	}

	if err := os.Rename(tmpPath, repoPath); err != nil {
		return nil, fmt.Errorf("unable to move the clone to %s: %w", repoPath, err)
	}
	log.Println("Cloning done")

	// Reopened at its final path: the repository returned by the clone still
	// refers to the temporary directory.
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open repo: %w", err)
	}
	return repo, nil
}

// tagFetchOptions fetches the tags forced and pruning: a tag deleted or moved
// upstream must not keep winning locally, which would wedge every later run on
// it.
func tagFetchOptions() *git.FetchOptions {
	return &git.FetchOptions{
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec("+refs/tags/*:refs/tags/*"),
		},
		Force: true,
		Prune: true,
	}
}

// originUrl returns the first URL of the origin remote.
func originUrl(repo *git.Repository) (string, error) {
	remote, err := repo.Remote(git.DefaultRemoteName)
	if err != nil {
		return "", err
	}
	if len(remote.Config().URLs) == 0 {
		return "", fmt.Errorf("remote %s has no URL", git.DefaultRemoteName)
	}
	return remote.Config().URLs[0], nil
}

// migrateLegacyClone moves a clone made by an older version, which keyed the
// directory by the URL basename alone, to its URL-keyed location. Only a clone
// whose origin is the configured URL is moved, so a directory shared by two
// same-named repositories goes to the one it actually belongs to.
func migrateLegacyClone(legacyPath, repoPath, url string) error {
	if legacyPath == repoPath {
		return nil
	}
	if _, err := os.Stat(repoPath); err == nil {
		return nil
	}
	if _, err := os.Stat(legacyPath); err != nil {
		return nil
	}

	repo, err := git.PlainOpen(legacyPath)
	if err != nil {
		return nil
	}
	if originUrl, err := originUrl(repo); err != nil || originUrl != url {
		return nil
	}

	log.Printf("Moving clone %s to %s\n", legacyPath, repoPath)
	if err := os.Rename(legacyPath, repoPath); err != nil {
		return fmt.Errorf("unable to move clone %s to %s: %w", legacyPath, repoPath, err)
	}
	return nil
}
