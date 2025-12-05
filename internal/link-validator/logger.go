package link_validator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"link-validator/pkg/config"
	"log/slog"
	"os"
)

type CustomHandler struct {
	writer io.Writer
	level  slog.Level
}

func InitLogger(cfg *config.Config) *CustomHandler {
	return &CustomHandler{
		writer: os.Stdout,
		level:  cfg.LogLevel.Level(),
	}
}

func (h *CustomHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *CustomHandler) Handle(ctx context.Context, record slog.Record) error {
	var levelStr string
	switch record.Level {
	case slog.LevelWarn:
		levelStr = "::warning::"
	case slog.LevelError:
		levelStr = "::error::"
	case slog.LevelInfo:
		levelStr = "INFO"
	case slog.LevelDebug:
		levelStr = "DEBUG"
	default:
		levelStr = record.Level.String()
	}

	// Collect all attributes
	attrs := make(map[string]any)
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	// Format output: LEVEL\tMESSAGE\tJSON_ATTRS
	var output string
	if len(attrs) > 0 {
		attrsJSON, _ := json.Marshal(attrs)
		output = fmt.Sprintf("%s\t%s\t%s\n", levelStr, record.Message, string(attrsJSON))
	} else {
		output = fmt.Sprintf("%s\t%s\n", levelStr, record.Message)
	}

	_, err := h.writer.Write([]byte(output))
	return err
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, return the same handler
	return h
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	// For simplicity, return the same handler
	return h
}
