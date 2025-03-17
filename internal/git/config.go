package git

import (
	"fmt"
	"os"
	"trx/internal/config"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type RepoConfig struct {
	Url  string
	Auth *Auth
}

type Auth struct {
	AuthMethod transport.AuthMethod
}

func NewRepoConfig(config config.GitRepo) (*RepoConfig, error) {
	if config.Url == "" {
		return nil, fmt.Errorf("git url not specified")
	}

	if config.Auth.BasicAuth != nil {
		auth, err := newBasicAuth(config.Auth.BasicAuth.Username, config.Auth.BasicAuth.Password)
		if err != nil {
			return nil, err
		}
		return &RepoConfig{
			Url:  config.Url,
			Auth: auth,
		}, nil
	}

	auth, err := newSshAuth(config.Auth.SshKeyPath, config.Auth.SshKeyPassword)
	if err != nil {
		return nil, err
	}
	return &RepoConfig{
		Url:  config.Url,
		Auth: auth,
	}, nil
}

func newBasicAuth(username, password string) (*Auth, error) {
	return &Auth{
		AuthMethod: &http.BasicAuth{
			Username: username,
			Password: password,
		},
	}, nil
}

func newSshAuth(key, password string) (*Auth, error) {
	if key == "" {
		return nil, nil
	}
	sshKey, _ := os.ReadFile(key)
	publicKey, err := ssh.NewPublicKeys("git", sshKey, password)
	if err != nil {
		return nil, fmt.Errorf("unable to get ssh public key: %w", err)
	}
	return &Auth{
		AuthMethod: publicKey,
	}, nil
}
