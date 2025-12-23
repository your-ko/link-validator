package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Vault struct {
	Name  string
	Urls  []string
	Token string
}

type Config struct {
	PAT            string
	CorpPAT        string
	DDApiKey       string
	DDAppKey       string
	LogLevel       slog.Level    `yaml:"logLevel"`
	CorpGitHubUrl  string        `yaml:"corpGitHubUrl"`
	FileMasks      []string      `yaml:"fileMasks"`
	Files          []string      `yaml:"files"`
	Exclude        []string      `yaml:"exclude"`
	LookupPath     string        `yaml:"lookupPath"`
	Timeout        time.Duration `yaml:"timeout"`
	IgnoredDomains []string      `yaml:"ignoredDomains"`
	Vaults         []Vault       `yaml:"vaults"`
	reader         io.Reader
}

// Default generates default config
func Default() *Config {
	return &Config{
		LogLevel:   slog.LevelInfo,
		LookupPath: ".",
		FileMasks:  []string{"*.md"},
		Timeout:    3 * time.Second,
		Vaults:     []Vault{},
	}
}

func (cfg *Config) WithReader(r io.Reader) *Config {
	if r != nil {
		cfg.reader = r
	}
	return cfg
}

// Load loads the config in the following sequence:
// Default < Config file < ENV variables
// Vault tokens are read from environment after all other merging is complete
func (cfg *Config) Load() (*Config, error) {
	var tmp *Config
	var err error
	if cfg.reader != nil {
		tmp, err = cfg.loadFromReader()
		if err != nil {
			return nil, err
		}
	}
	if tmp != nil {
		cfg.merge(tmp)
	}
	tmp, err = readFromEnv()
	if err != nil {
		return nil, err
	}
	cfg.merge(tmp)

	// After all merging, populate vault tokens from environment variables
	if len(cfg.Vaults) > 0 {
		cfg.Vaults = readVaultTokensFromEnv(cfg.Vaults)
	}

	return cfg, nil
}

func (cfg *Config) loadFromReader() (*Config, error) {
	decoder := yaml.NewDecoder(cfg.reader)
	decoder.KnownFields(true)
	tmp := &Config{}
	err := decoder.Decode(tmp)
	if err != nil {
		// Check if this is an empty file or no data
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("can't decode config: %w", err)
	}
	return tmp, nil
}

func readFromEnv() (*Config, error) {
	cfg := &Config{}

	// Only set values if environment variables are actually set
	if corpURL := GetEnv("CORP_URL", ""); corpURL != "" {
		cfg.CorpGitHubUrl = corpURL
	}
	if pat := GetEnv("PAT", ""); pat != "" {
		cfg.PAT = pat
	}
	if ddApiKey := GetEnv("DD_API_KEY", ""); ddApiKey != "" {
		cfg.DDApiKey = ddApiKey
	}
	if ddAppKey := GetEnv("DD_APP_KEY", ""); ddAppKey != "" {
		cfg.DDAppKey = ddAppKey
	}
	if corpPAT := GetEnv("CORP_PAT", ""); corpPAT != "" {
		cfg.CorpPAT = corpPAT
	}
	if LogLevelStr := GetEnv("LOG_LEVEL", ""); LogLevelStr != "" {
		slogLevel, err := ParseLevel(LogLevelStr)
		if err != nil {
			return nil, fmt.Errorf("can't parse logLevel: '%s', error: %w", LogLevelStr, err)
		}
		cfg.LogLevel = slogLevel
	}
	if fileMasks := GetEnv("FILE_MASKS", ""); fileMasks != "" {
		cfg.FileMasks = strings.Split(strings.TrimSuffix(fileMasks, ","), ",")
	}
	if files := GetEnv("FILES", ""); files != "" {
		cfg.Files = strings.Split(strings.TrimSuffix(files, ","), ",")
	}
	if exclude := GetEnv("EXCLUDE", ""); exclude != "" {
		cfg.Exclude = strings.Split(strings.TrimSuffix(exclude, ","), ",")
	}
	if lookupPath := GetEnv("LOOKUP_PATH", "."); lookupPath != "" {
		cfg.LookupPath = lookupPath
	}
	if timeoutStr := GetEnv("TIMEOUT", ""); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid duration value: %s", timeoutStr)
		}
		cfg.Timeout = timeout
	}
	if ignoredDomainsStr := GetEnv("IGNORED_DOMAINS", ""); ignoredDomainsStr != "" {
		ignoredDomains := strings.Split(strings.TrimSuffix(ignoredDomainsStr, ","), ",")

		for i, s := range ignoredDomains {
			ignoredDomains[i] = strings.ToLower(s)
		}
		cfg.IgnoredDomains = ignoredDomains
	}
	// Note: Vault tokens are handled in the end phase
	return cfg, nil
}

// readVaultTokensFromEnv reads vault tokens from environment variables for configured vaults
func readVaultTokensFromEnv(configVaults []Vault) []Vault {
	if len(configVaults) == 0 {
		return []Vault{}
	}

	vaults := make([]Vault, len(configVaults))
	copy(vaults, configVaults)

	for i := range vaults {
		tokenKey := "VAULT_TOKEN_" + strings.ToUpper(vaults[i].Name)
		if token := os.Getenv(tokenKey); token != "" {
			vaults[i].Token = token
		} else {
			slog.Error("Missing Vault token for %s", slog.String("vault", vaults[i].Name))
		}
	}
	return vaults
}

// merge merges this config with another config
// if another config has empty values, then original values are not overwritten
func (cfg *Config) merge(config *Config) {
	defCfg := Default()
	if config == nil {
		return
	}
	if config.CorpGitHubUrl != defCfg.CorpGitHubUrl {
		cfg.CorpGitHubUrl = config.CorpGitHubUrl
	}
	if config.CorpPAT != defCfg.CorpPAT {
		cfg.CorpPAT = config.CorpPAT
	}
	if config.PAT != defCfg.PAT {
		cfg.PAT = config.PAT
	}
	if config.DDApiKey != "" {
		cfg.DDApiKey = config.DDApiKey
	}
	if config.DDAppKey != "" {
		cfg.DDAppKey = config.DDAppKey
	}
	if config.LogLevel != defCfg.LogLevel {
		cfg.LogLevel = config.LogLevel
	}
	if config.LookupPath != defCfg.LookupPath && config.LookupPath != "" {
		cfg.LookupPath = config.LookupPath
	}
	if config.Timeout != 0 {
		cfg.Timeout = config.Timeout
	}
	if len(config.FileMasks) != 0 {
		cfg.FileMasks = config.FileMasks
	}
	if len(config.Files) != 0 {
		cfg.Files = config.Files
	}
	if len(config.Exclude) != 0 {
		cfg.Exclude = config.Exclude
	}
	if len(config.IgnoredDomains) != 0 {
		cfg.IgnoredDomains = config.IgnoredDomains
	}
	if len(config.Vaults) != 0 {
		cfg.Vaults = config.Vaults
	}
}

func GetEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.ReplaceAll(val, " ", "")
	}
	return defaultValue
}

func ParseLevel(s string) (slog.Level, error) {
	var level slog.Level
	var err = level.UnmarshalText([]byte(s))
	return level, err
}
