package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type validator interface {
	validate() error
}

type Config struct {
	LogLevel   slog.Level       `yaml:"logLevel"`
	FileMasks  []string         `yaml:"fileMasks"`
	Files      []string         `yaml:"files"`
	Exclude    []string         `yaml:"exclude"`
	LookupPath string           `yaml:"lookupPath"`
	Timeout    time.Duration    `yaml:"timeout"`
	Validators ValidatorsConfig `yaml:"validators"`
}

type ValidatorConfig struct {
	Enabled bool `yaml:"enabled"`
}

func (cfg ValidatorConfig) validate() error {
	return nil
}

type GitHubConfig struct {
	Enabled       bool `yaml:"enabled"`
	PAT           string
	CorpPAT       string
	CorpGitHubUrl string `yaml:"corpGitHubUrl"`
}

func (cfg GitHubConfig) validate() error {
	if cfg.CorpGitHubUrl != "" {
		if cfg.CorpPAT == "" {
			return errors.New("it seems you set CORP_URL but didn't provide CORP_PAT. Expect false negatives because the " +
				"link-validator won't be able to fetch corl github without token")
		}
	}
	return nil
}

type DataDogConfig struct {
	Enabled bool `yaml:"enabled"`
	ApiKey  string
	AppKey  string
}

func (cfg DataDogConfig) validate() error {
	if cfg.Enabled && (cfg.ApiKey == "" || cfg.AppKey == "") {
		return errors.New("datadog validator is enabled but DD_API_KEY/DD_APP_KEY are not set")
	}
	return nil
}

type HttpConfig struct {
	Enabled        bool     `yaml:"enabled"`
	IgnoredDomains []string `yaml:"ignoredDomains"`
}

func (cfg HttpConfig) validate() error {
	return nil
}

type VaultValidatorConfig struct {
	Name  string   `yaml:"name"`
	Urls  []string `yaml:"urls"`
	Token string
}

func (cfg VaultValidatorConfig) validate() error {
	if cfg.Token == "" {
		return fmt.Errorf("vault '%s' validator is enabled but VAULT_TOKEN_%s is not set", cfg.Name, strings.ToUpper(cfg.Name))
	}
	return nil
}

type ValidatorsConfig struct {
	GitHub    GitHubConfig           `yaml:"github"`
	DataDog   DataDogConfig          `yaml:"datadog"`
	LocalPath ValidatorConfig        `yaml:"localPath"`
	Vaults    []VaultValidatorConfig `yaml:"vaults"`
	HTTP      HttpConfig             `yaml:"http"`
}

func (v ValidatorsConfig) validate() []error {
	validators := []validator{
		v.GitHub,
		v.DataDog,
		v.LocalPath,
		v.HTTP,
	}

	var result []error
	for _, validator := range validators {
		if err := validator.validate(); err != nil {
			result = append(result, err)
		}
	}
	for _, vault := range v.Vaults {
		if err := vault.validate(); err != nil {
			result = append(result, err)
		}
	}
	return result
}

// Default generates default config
func Default() *Config {
	return &Config{
		LogLevel:   slog.LevelInfo,
		LookupPath: ".",
		FileMasks:  []string{"*.md"},
		Timeout:    3 * time.Second,
		Validators: ValidatorsConfig{Vaults: []VaultValidatorConfig{}},
	}
}
