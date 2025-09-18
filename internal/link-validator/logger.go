package link_validator

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"strings"
)

// Init Create a production encoder config (JSON, ISO8601 timestamps)
func Init(logLevel zapcore.Level) *zap.Logger {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "", // omit timestamp, GitHub adds its own
		LevelKey:       "level",
		NameKey:        "",
		CallerKey:      "", // hide caller for cleaner output
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    ghActionsLevelEncoder, // GH annotations
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		logLevel,
	)

	return zap.New(core, zap.AddStacktrace(zapcore.PanicLevel))
}

// LogLevel reads LOG_LEVEL and defaults to info.
func LogLevel() zapcore.Level {
	val := os.Getenv("LOG_LEVEL")
	if val == "" {
		return zapcore.InfoLevel
	}
	var lvl zapcore.Level
	if err := lvl.Set(strings.ToLower(val)); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "invalid LOG_LEVEL=%q, using info\n", val)
		return zapcore.InfoLevel
	}
	return lvl
}

// ghActionsLevelEncoder adds ::error:: / ::warning:: prefixes for GH Actions.
func ghActionsLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		enc.AppendString("::error::")
	case zapcore.WarnLevel:
		enc.AppendString("::warning::")
	case zapcore.InfoLevel:
		enc.AppendString("INFO")
	case zapcore.DebugLevel:
		enc.AppendString("DEBUG")
	default:
		enc.AppendString(strings.ToUpper(l.String()))
	}
}
