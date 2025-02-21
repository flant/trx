package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type Config struct {
	Repo    GitRepo           `mapstructure:"repo" validate:"required"`
	Quorums []Quorum          `mapstructure:"quorums" validate:"required,min=1"`
	Env     map[string]string `mapstructure:"env"`

	Hooks             *Hooks   `mapstructure:"hooks,omitempty"`
	InitLastPublished string   `mapstructure:"initial_last_published_git_commit"`
	Commands          []string `mapstructure:"commands"`
}

type GitRepo struct {
	Url                     string      `mapstructure:"url" validate:"required"`
	Auth                    GitRepoAuth `mapstructure:"auth"`
	InitialLastProcessedTag string      `mapstructure:"initialLastProcessedTag"`
	ConfigFile              string      `mapstructure:"configFile"`
}

type GitRepoAuth struct {
	SshKeyPath     string     `mapstructure:"sshKeyPath"`
	SshKeyPassword string     `mapstructure:"sshKeyPassword"`
	BasicAuth      *BasicAuth `mapstructure:"basic"`
}

type BasicAuth struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type Quorum struct {
	Name             *string  `mapstructure:"name,omitempty"`
	MinNumberOfKeys  int      `mapstructure:"minNumberOfKeys" validate:"required,gt=0"`
	GPGKeys          []string `mapstructure:"gpgKeys"`
	GPGKeyFilesPaths []string `mapstructure:"gpgKeyPaths"`
}

type Hooks struct {
	OnCommandSuccess *[]string `mapstructure:"onCommandSuccess,omitempty"`
	OnCommandFailure *[]string `mapstructure:"onCommandFailure,omitempty"`
	OnCommandSkipped *[]string `mapstructure:"onCommandSkipped,omitempty"`
	OnQuorumFailure  *[]string `mapstructure:"onQuorumFailure,omitempty"`
	OnCommandStarted *[]string `mapstructure:"onCommandStarted,omitempty"`
}

func NewConfig(configPath string) (*Config, error) {
	config := &Config{}

	err := loadConfig(configPath, _default, config, config.Validate)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func _default() {
	viper.SetConfigName("trx")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
}

func (config *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return err
	}

	if err := validateGitRepoPath(config.Repo); err != nil {
		return err
	}

	if err := validateQuorums(config.Quorums); err != nil {
		return err
	}

	return nil
}

func validateGitRepoPath(repo GitRepo) error {
	var sshGitRegex = regexp.MustCompile(`^git@(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}(:[a-zA-Z0-9-_/]+\.git)$`)
	var httpsGitRegex = regexp.MustCompile(`^https?://(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}(?:/[^\s]*)?\.git$`)

	switch {
	case sshGitRegex.MatchString(repo.Url):
		if repo.Auth.BasicAuth != nil {
			return fmt.Errorf("unable to use BasicAuth with SSH. should be only used when cloning by http/https")
		}
		if len(repo.Auth.SshKeyPath) > 0 {
			if err := fileExists(repo.Auth.SshKeyPath); err != nil {
				return fmt.Errorf("unable to validate ssh key path: %w", err)
			}
		}
		return nil

	case httpsGitRegex.MatchString(repo.Url):
		if len(repo.Auth.SshKeyPath) > 0 {
			return fmt.Errorf("unable to use ssh keys when cloning repo by https. should be only used when cloning by ssh")
		}
		return nil
	default:
		return fmt.Errorf("invalid Git repository URL: must be SSH (git@...) or HTTPS (https://...)")
	}
}

func validateQuorums(quorums []Quorum) error {
	for _, q := range quorums {
		if q.MinNumberOfKeys < 1 {
			return fmt.Errorf("quorum size needs to be greater or equal 1")
		}
		n := len(q.GPGKeyFilesPaths) + len(q.GPGKeys)
		if n < q.MinNumberOfKeys {
			return fmt.Errorf("number of GPG keys is less then number of minimum GPG keys. specified: %d, minimum number: %d", n, q.MinNumberOfKeys)
		}

		if err := validateKeyFilePath(q.GPGKeyFilesPaths); err != nil {
			return err
		}
	}
	return nil
}

func validateKeyFilePath(path []string) error {
	if len(path) == 0 {
		return nil
	}
	for _, p := range path {
		if err := fileExists(p); err != nil {
			return err
		}
	}
	return nil
}

func fileExists(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error stat key file path: %w", err)
	}
	return nil
}

func loadConfig(configPath string, defaultFunc func(), config interface{}, validate func() error) error {
	if configPath == "" {
		defaultFunc()
	} else {
		viper.SetConfigFile(configPath)
	}

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("unable to read config: %w", err)
	}

	decoderConfig := &mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      config,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return fmt.Errorf("unable to create config decoder: %w", err)
	}

	if err = decoder.Decode(viper.AllSettings()); err != nil {
		return fmt.Errorf("unable to decode config: %w", err)
	}

	if err = validate(); err != nil {
		return fmt.Errorf("config validation error: %w", err)
	}

	return nil
}
