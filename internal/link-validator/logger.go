package link_validator

import (
	"link-validator/pkg/config"
	"log/slog"
	"os"
)

func InitLogger(cfg *config.Config) *slog.TextHandler {
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
						return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("Warning:")}
					}
					return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("Warning")}
				case "ERROR":
					// GitHub Actions error command with color
					if os.Getenv("GITHUB_ACTIONS") == "true" {
						return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("Error:")}
					}
					return slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("Error")}
				}
			}
			return a
		},
	})
	return textHandler

}
