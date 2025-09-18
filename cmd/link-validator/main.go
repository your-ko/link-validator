package main

import (
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"link-validator/internal/link-validator"
	"os"
	"strings"
)

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

	fileMasks := strings.Split(*flag.String("FILE_MASKS", GetEnv("FILE_MASKS", "*.md"), "File masks."), ",")
	path := *flag.String("LOOKUP_PATH", GetEnv("LOOKUP_PATH", "."), "Lookup file.")
	pat := *flag.String("PAT", GetRequiredEnv("PAT"), "GitHub PAT. Used to get access to GitHub.")
	baseUrl := *flag.String("BASE_URL", GetEnv("BASE_URL", "https://github.com"), "GitHub BASE URL.")

	baseUrl = strings.TrimSpace(strings.ToLower(baseUrl))

	logger.Info("Starting Link Validator",
		zap.String("version", link_validator.Version.Version),
		zap.String("build date", link_validator.Version.BuildDate),
		zap.String("git commit", link_validator.Version.GitCommit),
	)
	logger.Debug("Running with parameters",
		zap.Strings("FILE_MASKS", fileMasks),
		zap.String("LOOKUP_PATH", path),
		zap.String("BASE_URL", baseUrl),
	)

	config := link_validator.Config{
		BaseUrl: baseUrl,
		Path:    path,
		PAT:     pat,
	}

	validator := link_validator.New(config)

	filesList, err := validator.GetFiles(path, fileMasks)
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
}

func GetEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.ReplaceAll(val, " ", "")
	}
	return defaultValue
}

func GetRequiredEnv(key string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.ReplaceAll(val, " ", "")
	}
	panic(fmt.Errorf("%s is not set", key))
}
