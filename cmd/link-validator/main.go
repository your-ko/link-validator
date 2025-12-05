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
		slog.Error("can't initialise, exiting", slog.Any("error", err))
		os.Exit(1)
	}

	// Custom handler for GitHub Actions integration
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// GitHub Actions workflow commands integration
			if a.Key == slog.LevelKey {
				switch a.Value.String() {
				case "WARN":
					// GitHub Actions warning command with color
					if os.Getenv("GITHUB_ACTIONS") == "true" {
						return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("::warning::")}
					}
					return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("Warning")}
				case "ERROR":
					// GitHub Actions error command with color
					if os.Getenv("GITHUB_ACTIONS") == "true" {
						return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("::error::")}
					}
					return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("Error")}
				}
			}
			return a
		},
	})

	sLogger := slog.New(textHandler)
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
			//slog.String("LOOKUP_PATH", cfg.LookupPath), // not implemented yet
			slog.Any("EXCLUDE", cfg.Exclude),
			slog.String("CORP_URL", cfg.CorpGitHubUrl),
			slog.Duration("TIMEOUT", cfg.Timeout),
		))

	validator, err := link_validator.New(cfg)
	if err != nil {
		slog.Error("can't start validation", slog.Any("err", err))
		os.Exit(1)
	}

	filesList, err := validator.GetFiles()
	if err != nil {
		slog.Error("Error generating file list", slog.Any("err", err))
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
