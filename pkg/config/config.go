package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load loads the config in the following sequence:
// Default < Config file < ENV variables
// If there is no config file, then it is skipped
func Load(reader io.Reader) (*Config, error) {
	cfg := Default()
	var tmp *Config
	var err error
	if reader == nil {
		tmp = Default()
	} else {
		tmp, err = loadFromReader(reader)
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

func loadFromReader(reader io.Reader) (*Config, error) {
	decoder := yaml.NewDecoder(reader)
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
		cfg.Validators.GitHub.CorpGitHubUrl = corpURL
	}
	if pat := GetEnv("PAT", ""); pat != "" {
		cfg.Validators.GitHub.PAT = pat
	}
	if corpPAT := GetEnv("CORP_PAT", ""); corpPAT != "" {
		cfg.Validators.GitHub.CorpPAT = corpPAT
	}
	if ddApiKey := GetEnv("DD_API_KEY", ""); ddApiKey != "" {
		cfg.Validators.DataDog.ApiKey = ddApiKey
	}
	if ddAppKey := GetEnv("DD_APP_KEY", ""); ddAppKey != "" {
		cfg.Validators.DataDog.AppKey = ddAppKey
	}
	if LogLevelStr := GetEnv("LOG_LEVEL", ""); LogLevelStr != "" {
		slogLevel, err := ParseLevel(LogLevelStr)
		if err != nil {
			return nil, fmt.Errorf("can't parse logLevel: '%s', error: %w", LogLevelStr, err)
		}
		cfg.LogLevel = &slogLevel
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
	if timeoutStr := GetEnv("TIMEOUT", "3s"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid duration value: %s", timeoutStr)
		}
		cfg.Timeout = timeout
	}
	if redirectStr := GetEnv("REDIRECTS", "3"); redirectStr != "" {
		redirects, err := strconv.Atoi(redirectStr)
		if err != nil {
			return nil, fmt.Errorf("invalid redirects value: %s", redirectStr)
		}
		cfg.Validators.HTTP.Redirects = redirects
	}
	if ignoredStr := GetEnv("IGNORE", ""); ignoredStr != "" {
		ignored := strings.Split(strings.TrimSuffix(ignoredStr, ","), ",")

		for i, s := range ignored {
			ignored[i] = strings.ToLower(s)
		}
		cfg.Validators.HTTP.Ignore = ignored
	}

	return cfg, nil
}

// merge merges this config with another config
// if another config has empty values, then original values are not overwritten
func (cfg *Config) merge(merge *Config) {
	if merge == nil {
		return
	}
	if merge.Validators.GitHub.Enabled {
		cfg.Validators.GitHub.Enabled = true
	}
	if merge.Validators.GitHub.CorpGitHubUrl != "" {
		cfg.Validators.GitHub.CorpGitHubUrl = merge.Validators.GitHub.CorpGitHubUrl
	}
	if merge.Validators.GitHub.CorpPAT != "" {
		cfg.Validators.GitHub.CorpPAT = merge.Validators.GitHub.CorpPAT
	}
	if merge.Validators.GitHub.PAT != "" {
		cfg.Validators.GitHub.PAT = merge.Validators.GitHub.PAT
	}

	if merge.Validators.DataDog.Enabled {
		cfg.Validators.DataDog.Enabled = true
	}
	if merge.Validators.DataDog.ApiKey != "" {
		cfg.Validators.DataDog.ApiKey = merge.Validators.DataDog.ApiKey
	}
	if merge.Validators.DataDog.AppKey != "" {
		cfg.Validators.DataDog.AppKey = merge.Validators.DataDog.AppKey
	}

	if merge.Validators.HTTP.Enabled {
		cfg.Validators.HTTP.Enabled = true
	}
	cfg.Validators.HTTP.Ignore = mergeSlices(cfg.Validators.HTTP.Ignore, merge.Validators.HTTP.Ignore)
	cfg.Validators.HTTP.Redirects = merge.Validators.HTTP.Redirects

	if merge.LookupPath != "" {
		cfg.LookupPath = merge.LookupPath
	}
	if merge.Timeout != 0 {
		cfg.Timeout = merge.Timeout
	}
	if merge.LogLevel != nil {
		cfg.LogLevel = merge.LogLevel
	}
	cfg.FileMasks = mergeSlices(cfg.FileMasks, merge.FileMasks)
	cfg.Files = mergeSlices(cfg.Files, merge.Files)
	cfg.Exclude = mergeSlices(cfg.Exclude, merge.Exclude)
}

func GetEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return defaultValue
}

func ParseLevel(s string) (slog.Level, error) {
	var level slog.Level
	var err = level.UnmarshalText([]byte(s))
	return level, err
}

func mergeSlices(left []string, right []string) []string {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	accu := make(map[string]bool)
	for i := range left {
		accu[left[i]] = true
	}
	for i := range right {
		accu[right[i]] = true
	}
	result := make([]string, 0, len(accu))
	for k := range accu {
		result = append(result, k)
	}
	return result
}

func (cfg *Config) Validate() []error {
	return cfg.Validators.validate()
}
