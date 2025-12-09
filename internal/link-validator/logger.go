package link_validator

import (
	"context"
	"encoding/json"
	"io"
	"link-validator/pkg/config"
	"log/slog"
	"os"
)

type CustomHandler struct {
	writer io.Writer
	level  slog.Level
	attrs  []slog.Attr
	group  string
}

func InitLogger(cfg *config.Config) *CustomHandler {
	return &CustomHandler{
		writer: os.Stderr,
		level:  cfg.LogLevel.Level(),
	}
}

func (h *CustomHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *CustomHandler) Handle(_ context.Context, record slog.Record) error {
	levelStr := map[slog.Level]string{
		slog.LevelWarn:  "::warning::",
		slog.LevelError: "::error::",
		slog.LevelInfo:  "INFO",
		slog.LevelDebug: "DEBUG",
	}[record.Level]

	if levelStr == "" {
		levelStr = record.Level.String()
	}

	attrs := make(map[string]any)

	// Add existing attributes
	for _, attr := range h.attrs {
		attrs[attr.Key] = attr.Value.Any()
	}

	// Add record attributes
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	// Format output
	output := levelStr + "\t" + record.Message
	if len(attrs) > 0 {
		if attrsJSON, err := json.Marshal(attrs); err == nil {
			output += "\t" + string(attrsJSON)
		}
	}
	output += "\n"

	_, err := h.writer.Write([]byte(output))
	return err
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	return &CustomHandler{
		writer: h.writer,
		level:  h.level,
		attrs:  append(append([]slog.Attr(nil), h.attrs...), attrs...),
		group:  h.group,
	}
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return &CustomHandler{
		writer: h.writer,
		level:  h.level,
		attrs:  h.attrs,
		group:  name,
	}
}
