package main

import (
	"context"
	"link-validator/internal/link-validator"
	"link-validator/pkg/config"
	"os"

	"go.uber.org/zap"
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

	logger.Info("Starting Link Validator",
		zap.String("version", Version),
		zap.String("build date", BuildDate),
		zap.String("git commit", GitCommit),
	)

	cfg := config.Default()
	if cfgReader, err := os.Open(".link-validator.yaml"); err == nil {
		cfg = cfg.WithReader(cfgReader)
		defer func() {
			_ = cfgReader.Close()
		}()
	} else {
		logger.Info("can't open config file .link-validator.yaml, skipping it")
	}
	cfg, err := cfg.Load()
	if err != nil {
		logger.Error("can't initialise, exiting", zap.Error(err))
		os.Exit(1)
	}

	if cfg.CorpGitHubUrl != "" && cfg.CorpPAT == "" {
		logger.Warn("it seems you set CORP_URL but didn't provide CORP_PAT. Expect false negatives because the " +
			"link won't be able to fetch corl github without pat")
	}

	logger.Debug("Running with parameters",
		zap.String("LOG_LEVEL", os.Getenv("LOG_LEVEL")),
		zap.Strings("FILE_MASKS", cfg.FileMasks),
		zap.Strings("FILES", cfg.Files),
		zap.Strings("IGNORED_DOMAINS", cfg.IgnoredDomains),
		//zap.String("LOOKUP_PATH", cfg.LookupPath), // not implemented yet
		zap.Strings("EXCLUDE_PATH", cfg.Exclude),
		zap.String("CORP_URL", cfg.CorpGitHubUrl),
		zap.Duration("TIMEOUT", cfg.Timeout),
	)

	validator := link_validator.New(cfg, logger)

	filesList, err := validator.GetFiles()
	if err != nil {
		logger.Fatal("Error generating file list", zap.Error(err))
	}
	logger.Debug("Found files", zap.Strings("files", filesList))

	ctx := context.Background()
	stats := validator.ProcessFiles(ctx, filesList, logger)
	if stats.Errors != 0 {
		logger.Error("Errors found:", zap.Int("errors", stats.Errors))
	}
	if stats.NotFoundLinks > 0 {
		logger.Error("Links not found", zap.Int("links", stats.NotFoundLinks))
	}
	logger.Info("Files processed", zap.Int("files", stats.Files))
	logger.Info("Links processed", zap.Int("links", stats.TotalLinks))
	logger.Info("Lines processed", zap.Int("lines", stats.Lines))

	if stats.Errors > 0 || stats.NotFoundLinks > 0 {
		os.Exit(1)
	}
}
