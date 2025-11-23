package config

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Config struct {
	PAT            string
	CorpPAT        string
	CorpGitHubUrl  string        `yaml:"corpGitHubUrl"`
	FileMasks      []string      `yaml:"fileMasks"`
	ExcludePath    string        `yaml:"excludePath"`
	LookupPath     string        `yaml:"lookupPath"`
	Timeout        time.Duration `yaml:"timeout"`
	IgnoredDomains []string      `yaml:"ignoredDomains"`
	reader         io.Reader
	logger         *zap.Logger
}

func Default() *Config {
	return &Config{
		FileMasks: []string{"*.md"},
		Timeout:   3 * time.Second,
	}
}

func (cfg *Config) WithReader(r io.Reader) *Config {
	cfg.reader = r
	return cfg
}

func (cfg *Config) Load() (*Config, error) {
	var tmp *Config
	var err error
	if cfg.reader != nil {
		tmp, err = cfg.loadFromReader()
		if err != nil {
			return nil, err
		}
	}
	cfg.merge(tmp)
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
	var tmp *Config
	if err := decoder.Decode(tmp); err != nil {
		return nil, fmt.Errorf("can't decode config: %w", err)
	}
	return cfg, nil
}

func readFromEnv() (*Config, error) {
	timeoutStr := GetEnv("TIMEOUT", "3s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid duration value: %s", timeoutStr)
	}
	ignoredDomains := strings.Split(GetEnv("IGNORED_DOMAINS", ""), ",")
	for i, s := range ignoredDomains {
		ignoredDomains[i] = strings.ToLower(s)
	}

	return &Config{
		CorpGitHubUrl:  GetEnv("CORP_URL", ""),
		PAT:            GetEnv("PAT", ""),
		CorpPAT:        strings.ToLower(GetEnv("CORP_PAT", "")),
		FileMasks:      strings.Split(GetEnv("FILE_MASKS", "*.md"), ","),
		IgnoredDomains: ignoredDomains,
		LookupPath:     GetEnv("LOOKUP_PATH", "."),
		Timeout:        timeout,
	}, nil
}

func (cfg *Config) merge(config *Config) {
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
