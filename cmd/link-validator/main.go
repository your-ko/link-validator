package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/fs"
	"link-validator/internal/git"
	"link-validator/internal/http"
	"link-validator/internal/local"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type LinkProcessor interface {
	// Process expects to actually process the received text from slack from the given user
	// TODO: return stat of processed links (good/errored, error)
	Process(ctx context.Context, url string, logger *zap.Logger) error

	Regex() *regexp.Regexp
}

var processors []LinkProcessor

func main() {
	logger := initLogger(getLogLevel())
	defer logger.Sync()
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
	logger.Debug("Staring link-validator", zap.String("version", Version.Version))

	fileMasks := strings.Split(*flag.String("FILE_MASKS", GetEnv("FILE_MASKS", "*.md"), "File masks."), ",")
	path := *flag.String("LOOKUP_PATH", GetEnv("LOOKUP_PATH", "."), "Lookup file.")
	pat := *flag.String("PAT", GetRequiredEnv("PAT"), "GitHub PAT. Used to get access to GitHub.")
	baseUrl := *flag.String("BASE_URL", GetEnv("BASE_URL", "https://github.com"), "GitHub BASE URL.")

	processors = make([]LinkProcessor, 0)
	processors = append(processors, git.New(baseUrl, pat))
	processors = append(processors, local.New(path))
	processors = append(processors, http.New(baseUrl))

	filesList, err := getFiles(path, fileMasks)
	if err != nil {
		logger.Fatal("Error generating file list", zap.Error(err))
	}

	stat, err := processFiles(filesList, logger)
	if err != nil {
		logger.Fatal("Error checking file", zap.Error(err))
	}
	fmt.Println(stat)

}

// TODO: return stats? So I can do a summary after the file is processed?
func processFiles(filesList []string, logger *zap.Logger) (interface{}, error) {
	ctx := context.Background() // TODO: fix context

	for _, fileName := range filesList {
		logger.Debug("processing file", zap.String("fileName", fileName))
		f, err := os.Open(fileName)
		if err != nil {
			logger.Error("Error opening file", zap.String("file", fileName), zap.Error(err))
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			links := processLine(line)
			for link, processor := range links {
				err := processor.Process(ctx, link, logger)
				if err != nil {
					// TODO
					return nil, err
				}
			}

			//if httpProcessor.Regex().MatchString(line) {
			//	err = httpProcessor.Process(line, fileName, logger)
			//	if err != nil {
			//		statusCodeError := &http.StatusCodeError{}
			//		if errors.As(err, &statusCodeError) {
			//			logger.Error("can't read the link", zap.String("fileName", fileName), zap.Int("line", lineNum), zap.Int("statusCode", statusCodeError.StatusCode), zap.String("link", statusCodeError.Link))
			//		} else {
			//			logger.Error("error processing the link", zap.Error(err))
			//		}
			//	}
			//}
		}
		logger.Debug("Processed: ", zap.Int("lines", lineNum), zap.String("fileName", fileName))

	}
	return nil, nil
}

func getFiles(root string, masks []string) ([]string, error) {
	var matchedFiles []string

	matchesAnyMask := func(name string) bool {
		for _, mask := range masks {
			matched, err := filepath.Match(mask, name)
			if err == nil && matched {
				return true
			}
		}
		return false
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Just skip files/dirs we can't read
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if matchesAnyMask(d.Name()) {
			matchedFiles = append(matchedFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return matchedFiles, nil
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

func processLine(line string) map[string]LinkProcessor {
	found := make(map[string]LinkProcessor, len(processors))
	for _, p := range processors {
		parts := p.Regex().FindAllString(line, -1)
		for _, part := range parts {
			found[part] = p
		}
	}
	return found
}

// Create a production encoder config (JSON, ISO8601 timestamps)
func initLogger(logLevel zapcore.Level) *zap.Logger {
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

	return zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))
}

// getLogLevel reads LOG_LEVEL and defaults to info.
func getLogLevel() zapcore.Level {
	val := os.Getenv("LOG_LEVEL")
	if val == "" {
		return zapcore.InfoLevel
	}
	var lvl zapcore.Level
	if err := lvl.Set(strings.ToLower(val)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid LOG_LEVEL=%q, using info\n", val)
		return zapcore.InfoLevel
	}
	return lvl
}

// ghActionsLevelEncoder adds ::error:: / ::warning:: prefixes for GH Actions.
func ghActionsLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		enc.AppendString("::error:: ERROR")
	case zapcore.WarnLevel:
		enc.AppendString("::warning:: WARN")
	case zapcore.InfoLevel:
		enc.AppendString("INFO")
	case zapcore.DebugLevel:
		enc.AppendString("DEBUG")
	default:
		enc.AppendString(strings.ToUpper(l.String()))
	}
}
