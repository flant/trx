package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"trx/internal/config"
)

type RepoConfig struct {
	Url      string
	Auth     *Auth
	RepoPath string
}

type Auth struct {
	AuthMethod transport.AuthMethod
}

func NewRepoConfig(config config.GitRepo) (*RepoConfig, error) {
	if config.Url == "" {
		return nil, fmt.Errorf("git url not specified")
	}

	home, err := HomeDir()
	if err != nil {
		return nil, err
	}

	var auth *Auth
	if config.Auth.BasicAuth != nil {
		auth = newBasicAuth(config.Auth.BasicAuth.Username, config.Auth.BasicAuth.Password)
	} else {
		auth, err = newSshAuth(config.Auth.SshKeyPath, config.Auth.SshKeyPassword)
		if err != nil {
			return nil, err
		}
	}

	return &RepoConfig{
		Url:      config.Url,
		Auth:     auth,
		RepoPath: filepath.Join(home, ".trx", RepoNameFromUrl(config.Url)),
	}, nil
}

func newBasicAuth(username, password string) *Auth {
	return &Auth{
		AuthMethod: &http.BasicAuth{
			Username: username,
			Password: password,
		},
	}
}

func newSshAuth(key, password string) (*Auth, error) {
	if key == "" {
		return nil, nil
	}
	sshKey, err := os.ReadFile(key)
	if err != nil {
		return nil, fmt.Errorf("unable to read ssh key %s: %w", key, err)
	}
	publicKey, err := ssh.NewPublicKeys("git", sshKey, password)
	if err != nil {
		return nil, fmt.Errorf("unable to get ssh public key: %w", err)
	}
	return &Auth{
		AuthMethod: publicKey,
	}, nil
}
