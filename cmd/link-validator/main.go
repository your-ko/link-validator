package main

import (
	"context"
	"flag"
	"go.uber.org/zap"
	"link-validator/internal/link-validator"
	"os"
	"strings"
	"time"
)

var GitCommit string
var Version string
var BuildDate string

func main() {
	logger := link_validator.Init(link_validator.LogLevel())
	defer func(logger *zap.Logger) {
		_ = logger.Sync()
	}(logger)

	// Panic guard to log stacktrace if app crashes
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic: application crashed",
				zap.Any("panic", r),
				zap.Stack("stack"),
			)
			os.Exit(1)
		}
	}()

	fileMasks := strings.Split(*flag.String("LV_FILE_MASKS", GetEnv("LV_FILE_MASKS", "*.md"), "File masks."), ",")
	// not used atm
	lookUpPath := *flag.String("LV_LOOKUP_PATH", GetEnv("LV_LOOKUP_PATH", "."), "Lookup path to validate local links. Useful if the repo is big and you want to focus only on some part if it.")
	// not used atm
	//excludePath := *flag.String("EXCLUDE_PATH", GetEnv("EXCLUDE_PATH", "."), "Exclude path. Don't validate some path")
	// not used atm
	//files := strings.Split(*flag.String("FILE_LIST", GetEnv("FILE_LIST", "."), "List of files to validate. Useful for PR validation, for example"), ",")
	pat := *flag.String("LV_PAT", GetEnv("LV_PAT", ""), "GitHub PAT. Used to get access to GitHub.")
	corpPat := *flag.String("LV_CORP_PAT", GetEnv("LV_CORP_PAT", ""), "Corporate GitHub PAT. Used to get access to the corporate GitHub.")
	corpGitHub := *flag.String("LV_CORP_URL", GetEnv("LV_CORP_URL", ""), "Corporate GitHub URL.")
	timeout := *flag.Duration("LV_TIMEOUT", envDuration("LV_TIMEOUT", 3*time.Second, logger), "HTTP request timeout")
	corpGitHub = strings.TrimSpace(strings.ToLower(corpGitHub))

	logger.Info("Starting Link Validator",
		zap.String("version", Version),
		zap.String("build date", BuildDate),
		zap.String("git commit", GitCommit),
	)
	logger.Debug("Running with parameters",
		zap.String("LV_LOG_LEVEL", os.Getenv("LV_LOG_LEVEL")),
		zap.Strings("LV_FILE_MASKS", fileMasks),
		zap.String("LV_LOOKUP_PATH", lookUpPath), // not implemented yet
		//zap.String("EXCLUDE_PATH", excludePath), // not implemented yet
		//zap.Strings("FILE_LIST", files),         // not implemented yet
		zap.String("LV_CORP_URL", corpGitHub),
		zap.Duration("LV_TIMEOUT", timeout),
	)

	config := link_validator.Config{
		CorpGitHubUrl: corpGitHub,
		//Path:          lookUpPath,
		PAT:        pat,
		CorpPAT:    corpPat,
		FileMasks:  fileMasks,
		LookupPath: lookUpPath,
		//ExcludePath:   excludePath,
		Timeout: timeout,
	}

	validator := link_validator.New(config, logger)

	filesList, err := validator.GetFiles(config)
	if err != nil {
		logger.Fatal("Error generating file list", zap.Error(err))
	}
	logger.Debug("Found files", zap.Strings("files", filesList))

	ctx := context.Background()
	stats := validator.ProcessFiles(ctx, filesList, logger)
	if stats.Errors != 0 {
		logger.Error("Errors found:", zap.Int("errors", stats.Errors))
	}
	if stats.NotFound > 0 {
		logger.Error("Links not found", zap.Int("links", stats.NotFound))
	}
	logger.Info("Files processed", zap.Int("files", stats.Files))
	logger.Info("Links processed", zap.Int("links", stats.Links))
	logger.Info("Lines processed", zap.Int("lines", stats.Lines))

	if stats.Errors > 0 || stats.NotFound > 0 {
		os.Exit(1)
	}
}

func envDuration(key string, def time.Duration, logger *zap.Logger) time.Duration {
	if str := os.Getenv(key); str != "" {
		if dur, err := time.ParseDuration(str); err == nil {
			return dur
		}
		logger.Error("invalid duration value", zap.String("key", key))
	}
	return def
}
func GetEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.ReplaceAll(val, " ", "")
	}
	return defaultValue
}
