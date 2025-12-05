package link_validator

import (
	"link-validator/pkg/config"
	"log/slog"
	"os"
)

func InitLogger(cfg *config.Config) *slog.TextHandler {
	// Custom handler for clean, readable format
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Remove timestamp completely
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}

			if a.Key == slog.LevelKey {
				switch a.Value.String() {
				case "WARN":
					return slog.Attr{Key: "", Value: slog.StringValue("::warning::")}
				case "ERROR":
					return slog.Attr{Key: "", Value: slog.StringValue("::error::")}
				case "INFO":
					return slog.Attr{Key: "", Value: slog.StringValue("INFO")}
				case "DEBUG":
					return slog.Attr{Key: "", Value: slog.StringValue("DEBUG")}
				}
			}
			return a
		},
	})
	return textHandler
}
