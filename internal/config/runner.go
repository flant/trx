package config

import (
	"fmt"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type RunnerConfig struct {
	Commands []string          `mapstructure:"commands"`
	Env      map[string]string `mapstructure:"env"`
}

func NewRunnerConfig(wd, configPath string) (*RunnerConfig, error) {
	config := &RunnerConfig{}

	if len(configPath) > 0 {
		configPath = filepath.Join(wd, configPath)
	}

	err := loadConfig(configPath, func() { _defaultRunner(wd) }, config, config.Validate)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func _defaultRunner(wd string) {
	viper.SetConfigName("trx")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(wd)
}

func (config *RunnerConfig) Validate() error {
	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return err
	}

	if len(config.Commands) == 0 {
		return fmt.Errorf("runner config error: no commands to run")
	}

	return nil
}
