package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PAT            string
	CorpPAT        string
	CorpGitHubUrl  string        `yaml:"corpGitHubUrl"`
	FileMasks      []string      `yaml:"fileMasks"`
	Files          []string      `yaml:"files"`
	ExcludePath    []string      `yaml:"excludePath"`
	LookupPath     string        `yaml:"lookupPath"`
	Timeout        time.Duration `yaml:"timeout"`
	IgnoredDomains []string      `yaml:"ignoredDomains"`
	reader         io.Reader
}

// Default generates default config
func Default() *Config {
	return &Config{
		LookupPath: ".",
		FileMasks:  []string{"*.md"},
		Timeout:    3 * time.Second,
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
// If there is no config file, then it is skipped
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
	if corpPAT := GetEnv("CORP_PAT", ""); corpPAT != "" {
		cfg.CorpPAT = corpPAT
	}
	if fileMasks := GetEnv("FILE_MASKS", ""); fileMasks != "" {
		cfg.FileMasks = strings.Split(strings.TrimSuffix(fileMasks, ","), ",")
	}
	if files := GetEnv("FILES", ""); files != "" {
		cfg.Files = strings.Split(strings.TrimSuffix(files, ","), ",")
	}
	if exclude := GetEnv("EXCLUDE_PATH", ""); exclude != "" {
		cfg.Files = strings.Split(strings.TrimSuffix(exclude, ","), ",")
	}
	if lookupPath := GetEnv("LOOKUP_PATH", ""); lookupPath != "" {
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

	return cfg, nil
}

// merge merges this config with another config
// if another config has empty values, then original values are not overwritten
func (cfg *Config) merge(config *Config) {
	if config == nil {
		return
	}
	if config.CorpGitHubUrl != "" {
		cfg.CorpGitHubUrl = config.CorpGitHubUrl
	}
	if config.CorpPAT != "" {
		cfg.CorpPAT = config.CorpPAT
	}
	if config.PAT != "" {
		cfg.PAT = config.PAT
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
	if len(config.IgnoredDomains) != 0 {
		cfg.IgnoredDomains = config.IgnoredDomains
	}
}

func GetEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.ReplaceAll(val, " ", "")
	}
	return defaultValue
}
