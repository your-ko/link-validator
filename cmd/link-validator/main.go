package main

import (
	"context"
	"link-validator/internal/link-validator"
	"link-validator/pkg/config"
	"log/slog"
	"os"

	"go.uber.org/zap"
)

var GitCommit string
var Version string
var BuildDate string

func main() {
	//logger := link_validator.Init(link_validator.LogLevel())

	sLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(sLogger)

	//defer func(logger *zap.Logger) {
	//	_ = logger.Sync()
	//}(logger)

	// Panic guard to log stacktrace if app crashes
	//defer func() {
	//	if r := recover(); r != nil {
	//		logger.Error("panic: application crashed",
	//			zap.Any("panic", r),
	//			zap.Stack("stack"),
	//		)
	//		os.Exit(1)
	//	}
	//}()

	slog.Info("Starting Link Validator",
		slog.String("version", Version),
		slog.String("build date", BuildDate),
		slog.String("git commit", GitCommit),
	)

	cfg := config.Default()
	if cfgReader, err := os.Open(".link-validator.yaml"); err == nil {
		cfg = cfg.WithReader(cfgReader)
		defer func() {
			_ = cfgReader.Close()
		}()
	} else {
		slog.Info("can't open config file .link-validator.yaml, skipping it")
	}
	cfg, err := cfg.Load()
	if err != nil {
		slog.Error("can't initialise, exiting", zap.Error(err))
		os.Exit(1)
	}

	if cfg.CorpGitHubUrl != "" && cfg.CorpPAT == "" {
		slog.Warn("it seems you set CORP_URL but didn't provide CORP_PAT. Expect false negatives because the " +
			"link won't be able to fetch corl github without pat")
	}

	slog.Debug("Running with parameters",
		//slog.Group()
		slog.String("LOG_LEVEL", os.Getenv("LOG_LEVEL")),
		slog.Any("FILE_MASKS", cfg.FileMasks),
		slog.Any("FILES", cfg.Files),
		slog.Any("IGNORED_DOMAINS", cfg.IgnoredDomains),
		//zap.String("LOOKUP_PATH", cfg.LookupPath), // not implemented yet
		slog.Any("EXCLUDE", cfg.Exclude),
		slog.String("CORP_URL", cfg.CorpGitHubUrl),
		slog.Duration("TIMEOUT", cfg.Timeout),
	)
	os.Exit(1)

	validator := link_validator.New(cfg)

	filesList, err := validator.GetFiles()
	if err != nil {
		slog.Error("Error generating file list", zap.Error(err))
		os.Exit(1)
	}
	slog.Debug("Found files", zap.Strings("files", filesList))

	ctx := context.Background()
	stats := validator.ProcessFiles(ctx, filesList)
	if stats.Errors != 0 {
		slog.Error("Errors found:", zap.Int("errors", stats.Errors))
	}
	if stats.NotFoundLinks > 0 {
		slog.Error("Links not found", zap.Int("links", stats.NotFoundLinks))
	}
	slog.Info("Files processed", zap.Int("files", stats.Files))
	slog.Info("Links processed", zap.Int("links", stats.TotalLinks))
	slog.Info("Lines processed", zap.Int("lines", stats.Lines))

	if stats.Errors > 0 || stats.NotFoundLinks > 0 {
		os.Exit(1)
	}
}
