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
}

func InitLogger(cfg *config.Config) *CustomHandler {
	return &CustomHandler{
		writer: os.Stdout,
		level:  cfg.LogLevel.Level(),
	}
}

func (h *CustomHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func convertValue(v slog.Value) any {
	if v.Kind() == slog.KindAny {
		if err, ok := v.Any().(error); ok {
			return err.Error()
		}
	}
	return v.Any()
}

func (h *CustomHandler) Handle(_ context.Context, record slog.Record) error {
	// Simplified level mapping
	levelStr := map[slog.Level]string{
		slog.LevelWarn:  "::warning::",
		slog.LevelError: "::error::",
		slog.LevelInfo:  "INFO",
		slog.LevelDebug: "DEBUG",
	}[record.Level]

	if levelStr == "" {
		levelStr = record.Level.String()
	}

	attrs := make(map[string]any, len(h.attrs)+record.NumAttrs())

	for _, attr := range h.attrs {
		attrs[attr.Key] = convertValue(attr.Value)
	}

	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = convertValue(a.Value)
		return true
	})

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
	return &CustomHandler{
		writer: h.writer,
		level:  h.level,
		attrs:  append(append([]slog.Attr(nil), h.attrs...), attrs...), // Safe copy + append
	}
}

func (h *CustomHandler) WithGroup(_ string) slog.Handler {
	// Groups not needed for your use case - return same handler
	return h
}
