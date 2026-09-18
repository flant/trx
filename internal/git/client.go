package git

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"trx/internal/config"
)

type GitClient struct {
	Repo     *git.Repository
	RepoPath string
}

func NewGitClient(ctx context.Context, cfg config.GitRepo) (*GitClient, error) {
	repoConf, err := NewRepoConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("new repo config error: %w", err)
	}

	repo, err := openGitRepo(ctx, repoConf)
	if err != nil {
		return nil, fmt.Errorf("open git repo error: %w", err)
	}

	return &GitClient{
		Repo:     repo,
		RepoPath: repoConf.RepoPath,
	}, nil
}

func (g *GitClient) GetTargetGitObject(t string) (*TargetGitObject, error) {
	to, err := getTagInfo(g, t)
	if err != nil {
		return nil, fmt.Errorf("get tag info error: %w", err)
	}

	if err := g.Checkout(to); err != nil {
		return nil, fmt.Errorf("checkout error: %w", err)
	}
	return to, nil
}

func getTagInfo(g *GitClient, tag string) (*TargetGitObject, error) {
	if tag != "" {
		return g.GetSpecificTag(tag)
	}
	return g.GetLastSemverTag()
}

type TargetGitObject struct {
	Tag    string
	Commit string
}

func (g *GitClient) Checkout(o *TargetGitObject) error {
	log.Printf("Checking out tag %s (%s)\n", o.Tag, o.Commit)
	worktree, err := g.Repo.Worktree()
	if err != nil {
		return fmt.Errorf("unable to get worktree: %w", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(o.Commit),
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("checkout error: %w", err)
	}
	return nil
}

func (g *GitClient) GetLastSemverTag() (*TargetGitObject, error) {
	tagRefs, err := g.Repo.Tags()
	if err != nil {
		return nil, err
	}

	var versions []*semver.Version
	tagMap := make(map[string]plumbing.ReferenceName)

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
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no semantic version tags found")
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))
	lastTag := versions[0].Original()
	refName := tagMap[lastTag]

	ref, err := g.Repo.Reference(refName, true)
	if err != nil {
		return nil, err
	}

	return &TargetGitObject{
		Tag:    lastTag,
		Commit: g.peel(ref.Hash()).String(),
	}, nil
}

func (g *GitClient) GetSpecificTag(tag string) (*TargetGitObject, error) {
	ref, err := g.Repo.Tag(tag)
	if err != nil {
		return nil, fmt.Errorf("tag %q not found: %w", tag, err)
	}
	return &TargetGitObject{
		Tag:    tag,
		Commit: g.peel(ref.Hash()).String(),
	}, nil
}

// peel resolves an annotated tag object to the commit it points at. A
// lightweight tag already points at the commit and is returned as is.
func (g *GitClient) peel(hash plumbing.Hash) plumbing.Hash {
	obj, err := g.Repo.Object(plumbing.TagObject, hash)
	if err != nil {
		return hash
	}
	if annotatedTag, ok := obj.(*object.Tag); ok {
		return annotatedTag.Target
	}
	return hash
}

func openGitRepo(ctx context.Context, r *RepoConfig) (*git.Repository, error) {
	repo, err := git.PlainOpen(r.RepoPath)
	switch {
	case err != nil:
		if !errors.Is(err, git.ErrRepositoryNotExists) {
			log.Printf("WARNING unable to open existing clone %s (%s), cloning anew\n", r.RepoPath, err)
		}
		repo, err = cloneGitRepo(ctx, r)
		if err != nil {
			return nil, err
		}
	case !hasOrigin(repo, r.Url):
		log.Printf("WARNING clone %s does not point at %s, cloning anew\n", r.RepoPath, r.Url)
		repo, err = cloneGitRepo(ctx, r)
		if err != nil {
			return nil, err
		}
	}

	log.Println("Fetching tags")
	fetchOptions := &git.FetchOptions{
		RemoteName: git.DefaultRemoteName,
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec("refs/tags/*:refs/tags/*"),
		},
		// Prune drops tags deleted upstream, which otherwise keep winning tag
		// selection forever. Updates are not forced: a tag moved upstream must
		// not silently replace the local one.
		Prune: true,
	}
	if r.Auth != nil {
		fetchOptions.Auth = r.Auth.AuthMethod
	}
	err = repo.FetchContext(ctx, fetchOptions)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, fmt.Errorf("unable to fetch tags: %w", err)
	}

	return repo, nil
}

// cloneGitRepo clones into a temporary directory next to the target and renames
// it into place, so that an interrupted clone never leaves a broken repository
// behind.
func cloneGitRepo(ctx context.Context, r *RepoConfig) (*git.Repository, error) {
	parentDir := filepath.Dir(r.RepoPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, fmt.Errorf("unable to create %s: %w", parentDir, err)
	}
	tmpDir, err := os.MkdirTemp(parentDir, ".clone-")
	if err != nil {
		return nil, fmt.Errorf("unable to create temporary clone directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneDir := filepath.Join(tmpDir, "repo")
	cloneOptions := &git.CloneOptions{URL: r.Url}
	if r.Auth != nil {
		cloneOptions.Auth = r.Auth.AuthMethod
	}

	log.Printf("Cloning %s into %s\n", r.Url, r.RepoPath)
	if _, err := git.PlainCloneContext(ctx, cloneDir, false, cloneOptions); err != nil {
		return nil, fmt.Errorf("unable to clone repo: %w", err)
	}

	if err := os.RemoveAll(r.RepoPath); err != nil {
		return nil, fmt.Errorf("unable to remove %s: %w", r.RepoPath, err)
	}
	if err := os.Rename(cloneDir, r.RepoPath); err != nil {
		return nil, fmt.Errorf("unable to move clone into %s: %w", r.RepoPath, err)
	}
	log.Println("Cloning done")

	return git.PlainOpen(r.RepoPath)
}

func hasOrigin(repo *git.Repository, url string) bool {
	remote, err := repo.Remote(git.DefaultRemoteName)
	if err != nil {
		return false
	}
	for _, u := range remote.Config().URLs {
		if u == url {
			return true
		}
	}
	return false
}
