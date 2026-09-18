package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type Config struct {
	Repo    GitRepo           `mapstructure:"repo" validate:"required"`
	Quorums []Quorum          `mapstructure:"quorums" validate:"required,min=1"`
	Env     map[string]string `mapstructure:"env"`

	Hooks    *Hooks   `mapstructure:"hooks,omitempty"`
	Commands []string `mapstructure:"commands"`
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

// defaultConfigPath is where the operator config is read from when no path is
// given, as the --config flag and the command help both promise.
const defaultConfigPath = "./trx.yaml"

func NewConfig(configPath string) (*Config, error) {
	config := &Config{}

	if configPath == "" {
		configPath = defaultConfigPath
	}

	err := loadConfig(configPath, nil, config, config.Validate)
	if err != nil {
		return nil, err
	}

	return config, nil
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
	sshGitRegex := regexp.MustCompile(`^git@(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}(:[a-zA-Z0-9-_/]+\.git)$`)
	httpsGitRegex := regexp.MustCompile(`^https?://(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}(?:/[^\s]*)?\.git$`)

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
		if err := validateKeyFilePath(q.GPGKeyFilesPaths); err != nil {
			return err
		}

		// Counted after the keys are parsed: a quorum is a number of distinct
		// key holders, not a number of config entries.
		n, err := validateGPGKeys(q)
		if err != nil {
			return err
		}
		if n < q.MinNumberOfKeys {
			return fmt.Errorf("number of GPG keys is less then number of minimum GPG keys. specified: %d, minimum number: %d", n, q.MinNumberOfKeys)
		}
	}
	return nil
}

// validateGPGKeys makes sure every trusted key is a parseable armored public
// key. Doing it here surfaces a misconfigured key as a config error instead of
// a quorum failure (which would also trigger the onQuorumFailure hook).
// It returns the number of key entries, and rejects a quorum that lists the
// same key twice: the verifier drops one entry per matching signature, so a
// duplicated entry lets a single key holder satisfy minNumberOfKeys > 1. An
// entry holding several keys still counts as one, for the same reason.
func validateGPGKeys(q Quorum) (int, error) {
	name := q.DisplayName()

	keys, err := q.AllGPGKeys()
	if err != nil {
		return 0, fmt.Errorf("quorum %q %w", name, err)
	}

	seenAt := make(map[string]string)
	for i, key := range keys {
		fingerprints, err := validateGPGKey(key)
		if err != nil {
			return 0, fmt.Errorf("quorum %q %s: %w", name, q.gpgKeySource(i), err)
		}
		for _, fp := range fingerprints {
			if prev, ok := seenAt[fp]; ok {
				return 0, fmt.Errorf("quorum %q %s: duplicates the GPG key %s already listed in %s. a quorum must be made of distinct keys", name, q.gpgKeySource(i), fp, prev)
			}
			seenAt[fp] = q.gpgKeySource(i)
		}
	}

	return len(keys), nil
}

// DisplayName names the quorum in logs and errors. The name is optional in the
// config, and dereferencing it unconditionally used to panic mid-run.
func (q Quorum) DisplayName() string {
	if q.Name == nil {
		return "<unnamed>"
	}
	return *q.Name
}

// AllGPGKeys returns the trusted keys of the quorum: the inline ones and the
// contents of every gpgKeyPaths file.
func (q Quorum) AllGPGKeys() ([]string, error) {
	keys := make([]string, 0, len(q.GPGKeys)+len(q.GPGKeyFilesPaths))
	for i, path := range q.GPGKeyFilesPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("gpgKeyPaths[%d]: unable to read key file: %w", i, err)
		}
		keys = append(keys, string(data))
	}
	return append(keys, q.GPGKeys...), nil
}

// gpgKeySource names the config field a key from AllGPGKeys came from.
func (q Quorum) gpgKeySource(i int) string {
	if i < len(q.GPGKeyFilesPaths) {
		return fmt.Sprintf("gpgKeyPaths[%d] (%s)", i, q.GPGKeyFilesPaths[i])
	}
	return fmt.Sprintf("gpgKeys[%d]", i-len(q.GPGKeyFilesPaths))
}

// validateGPGKey returns the primary fingerprints of the keys in an entry.
func validateGPGKey(key string) ([]string, error) {
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(key))
	if err != nil {
		return nil, fmt.Errorf("invalid GPG public key: %w", err)
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("invalid GPG public key: no public key found")
	}
	fingerprints := make([]string, 0, len(entities))
	for _, e := range entities {
		if e.PrivateKey != nil {
			return nil, fmt.Errorf("invalid GPG public key: key %X is a private key", e.PrimaryKey.KeyId)
		}
		fingerprints = append(fingerprints, fmt.Sprintf("%X", e.PrimaryKey.Fingerprint))
	}
	return fingerprints, nil
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
		if defaultFunc == nil {
			return fmt.Errorf("config path is not specified")
		}
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
