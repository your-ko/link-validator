package link_validator

import (
	"bufio"
	"context"
	"errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/fs"
	"link-validator/pkg/errs"
	"link-validator/pkg/external"
	"link-validator/pkg/intern"
	"link-validator/pkg/local"
	"os"
	"path/filepath"
	"time"
)

type LinkProcessor interface {
	Process(ctx context.Context, url string, logger *zap.Logger) error

	ExtractLinks(line string) []string
}

type Stats struct {
	Lines    int
	Links    int
	Errors   int
	NotFound int
	Files    int
}

type LinkValidador struct {
	processors []LinkProcessor
}

type Config struct {
	CorpGitHubUrl string
	Path          string
	PAT           string
	FileMasks     []string
	ExcludePath   string
	LookupPath    string
}

func New(config Config) LinkValidador {
	processors := make([]LinkProcessor, 0)
	if config.CorpGitHubUrl != "" {
		processors = append(processors, intern.New(config.CorpGitHubUrl, config.PAT))
	}
	processors = append(processors, local.New())
	processors = append(processors, external.New(config.CorpGitHubUrl))
	return LinkValidador{processors}
}

func (v *LinkValidador) ProcessFiles(ctx context.Context, filesList []string, logger *zap.Logger) Stats {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	stats := Stats{}

	for _, fileName := range filesList {
		logger.Debug("Processing file:", zap.String("fileName", fileName))
		stats.Files++
		f, err := os.Open(fileName)
		if err != nil {
			logger.Error("Error opening file", zap.String("file", fileName), zap.Error(err))
			continue
		}
		defer f.Close()
		lines := 0
		linksFound := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			links := v.processLine(line)
			for link, processor := range links {
				err := processor.Process(ctx, link, logger)
				linksFound++
				if err != nil {
					var notFound errs.NotFoundError
					var empty errs.EmptyBodyError
					if errors.As(err, &notFound) {
						logger.Warn("link not found", zap.String("link", notFound.Error()), zap.String("filename", fileName), zap.Int("line", lines))
						stats.NotFound++
					} else if errors.As(err, &empty) {
						logger.Warn("link not found", zap.String("link", empty.Error()), zap.String("filename", fileName), zap.Int("line", lines))
						stats.NotFound++
					} else {
						stats.Errors++
						logger.Warn("error validating link", zap.String("link", link), zap.Error(err))
					}
					logger.Debug("link validation successful", zap.String("link", link), zap.String("filename", fileName), zap.Int("line", lines))
				}
			}
			lines++
		}
		stats.Lines = stats.Lines + lines
		stats.Links = stats.Links + linksFound

		if zapcore.DebugLevel == logger.Level() {
			logger.Debug("Processed: ", zap.Int("lines", lines), zap.Int("links", linksFound), zap.String("fileName", fileName))
		} else {
			logger.Info("Processed: ", zap.String("fileName", fileName))
		}
	}
	return stats
}

func (v *LinkValidador) GetFiles(config Config) ([]string, error) {
	var matchedFiles []string

	matchesAnyMask := func(name string) bool {
		for _, mask := range config.FileMasks {
			matched, err := filepath.Match(mask, name)
			if err == nil && matched {
				return true
			}
		}
		return false
	}

	err := filepath.WalkDir(config.LookupPath, func(path string, d fs.DirEntry, err error) error {
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

func (v *LinkValidador) processLine(line string) map[string]LinkProcessor {
	found := make(map[string]LinkProcessor, len(v.processors))
	for _, p := range v.processors {
		links := p.ExtractLinks(line)
		for _, link := range links {
			found[link] = p
		}
	}
	return found
}
