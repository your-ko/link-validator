package main

import (
	"context"
	"link-validator/internal/link-validator"
	"link-validator/pkg/config"
	"log/slog"
	"os"
)

var GitCommit string
var Version string
var BuildDate string

func main() {

	cfg := config.Default()
	if cfgReader, err := os.Open(".link-validator.yaml"); err == nil {
		cfg = cfg.WithReader(cfgReader)
		defer func() {
			_ = cfgReader.Close()
		}()
	} else {
		slog.Warn("can't open config file .link-validator.yaml, skipping it")
	}
	cfg, err := cfg.Load()
	if err != nil {
		slog.With("error", err).Error("can't initialise, exiting")
		os.Exit(1)
	}

	sLogger := slog.New(link_validator.InitLogger(cfg))
	slog.SetDefault(sLogger)

	slog.Info("Starting Link Validator",
		slog.Group("version",
			slog.String("v", Version),
			slog.String("build date", BuildDate),
			slog.String("git commit", GitCommit),
		))

	if cfg.CorpGitHubUrl != "" && cfg.CorpPAT == "" {
		slog.Warn("it seems you set CORP_URL but didn't provide CORP_PAT. Expect false negatives because the " +
			"link won't be able to fetch corl github without pat")
	}

	slog.Debug("Running with",
		slog.Group("config",
			slog.String("LOG_LEVEL", os.Getenv("LOG_LEVEL")),
			slog.Any("FILE_MASKS", cfg.FileMasks),
			slog.Any("FILES", cfg.Files),
			slog.Any("IGNORED_DOMAINS", cfg.IgnoredDomains),
			slog.String("LOOKUP_PATH", cfg.LookupPath),
			slog.Any("EXCLUDE", cfg.Exclude),
			slog.String("CORP_URL", cfg.CorpGitHubUrl),
			slog.Duration("TIMEOUT", cfg.Timeout),
		))

	validator, err := link_validator.New(cfg)
	if err != nil {
		slog.With("error", err).Error("can't start validation")
		os.Exit(1)
	}

	filesList, err := validator.GetFiles()
	if err != nil {
		slog.With("error", err).Error("Error generating file list")
		os.Exit(1)
	}
	slog.Debug("Found files", slog.Any("files", filesList))

	ctx := context.Background()
	stats := validator.ProcessFiles(ctx, filesList)
	if stats.Errors != 0 {
		slog.Error("Errors found:", slog.Int("errors", stats.Errors))
	}
	slog.Info("Files processed", slog.Int("files", stats.Files))
	slog.Info("Links processed", slog.Int("links", stats.TotalLinks))
	slog.Info("Lines processed", slog.Int("lines", stats.Lines))
	if stats.NotFoundLinks > 0 {
		slog.Error("Links not found", slog.Int("links", stats.NotFoundLinks))
	}

	if stats.Errors > 0 || stats.NotFoundLinks > 0 {
		os.Exit(1)
	}
}
