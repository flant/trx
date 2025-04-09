package git

import (
	"errors"
	"fmt"
	"log"
	"os"
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

func NewGitClient(cfg config.GitRepo) (*GitClient, error) {
	repoConf, err := NewRepoConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("new repo config error: %w", err)
	}

	repo, err := openGitRepo(repoConf)
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

	err = g.Checkout(to)
	if err != nil {
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
	log.Printf("Got last tag %s. Perform checkout\n", o.Tag)
	tagRef, err := g.Repo.Tag(o.Tag)
	if err != nil {
		return fmt.Errorf("tag not found: %w", err)
	}
	tagHash := tagRef.Hash()
	tagObj, err := g.Repo.Object(plumbing.TagObject, tagHash)
	if err == nil {
		annotatedTag, ok := tagObj.(*object.Tag)
		if ok {
			tagHash = annotatedTag.Target
		}
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
		Commit: ref.Hash().String(),
	}, nil
}

func (g *GitClient) GetSpecificTag(tag string) (*TargetGitObject, error) {
	ref, err := g.Repo.Tag(tag)
	if err != nil {
		return nil, fmt.Errorf("tag %q not found: %w", tag, err)
	}
	return &TargetGitObject{
		Tag:    tag,
		Commit: ref.Hash().String(),
	}, nil
}

func openGitRepo(r *RepoConfig) (*git.Repository, error) {
	var repo *git.Repository
	if _, err := os.Stat(r.RepoPath); os.IsNotExist(err) {
		cloneOptions := &git.CloneOptions{URL: r.Url}
		if r.Auth != nil {
			cloneOptions.Auth = r.Auth.AuthMethod
		}

		log.Printf("Cloning %s into %s\n", r.Url, r.RepoPath)
		repo, err = git.PlainClone(r.RepoPath, false, cloneOptions)
		if err != nil {
			return nil, fmt.Errorf("unable to clone repo: %w", err)
		}
		log.Println("Cloning done")
	} else {
		repo, err = git.PlainOpen(r.RepoPath)
		if err != nil {
			return nil, fmt.Errorf("unable to open repo: %w", err)
		}
	}

	log.Println("Fetching tags")
	fetchOptions := &git.FetchOptions{
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec("refs/tags/*:refs/tags/*"),
		},
	}
	if r.Auth != nil {
		fetchOptions.Auth = r.Auth.AuthMethod
	}
	err := repo.Fetch(fetchOptions)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, fmt.Errorf("unable to fetch tags: %w", err)
	}

	return repo, nil
}
