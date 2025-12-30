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
	var cfg *config.Config
	var err error

	cfgReader, err := os.Open(".link-validator.yaml")
	if err == nil {
		defer func() {
			_ = cfgReader.Close()
		}()
		cfg, err = config.Load(cfgReader)
		if err != nil {
			slog.With("error", err).Error("can't initialise, exiting")
			os.Exit(1)
		}
	} else {
		slog.Warn("can't open config file .link-validator.yaml, using default and ENV variables")
		cfg, err = config.Load(nil)
		if err != nil {
			slog.With("error", err).Error("can't initialise config from ENV variables, exiting")
			os.Exit(1)
		}
	}

	sLogger := slog.New(link_validator.InitLogger(cfg))
	slog.SetDefault(sLogger)

	slog.Info("Starting Link Validator",
		slog.String("version", Version),
		slog.String("build date", BuildDate),
		slog.String("git commit", GitCommit),
	)

	res := cfg.Validate()
	for _, err = range res {
		slog.With("error", err).Error("initialisation error:")
		os.Exit(1)
	}

	slog.Debug("Running with",
		slog.String("LOG_LEVEL", os.Getenv("LOG_LEVEL")),
		slog.String("CORP_URL", cfg.Validators.GitHub.CorpGitHubUrl),
		slog.String("LOOKUP_PATH", cfg.LookupPath),
		slog.Any("FILE_MASKS", cfg.FileMasks),
		slog.Duration("TIMEOUT", cfg.Timeout),
		slog.Any("EXCLUDE", cfg.Exclude),
		slog.Any("FILES", cfg.Files),
	)

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
