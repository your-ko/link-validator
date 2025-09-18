package link_validator

import (
	"bufio"
	"context"
	"errors"
	"go.uber.org/zap"
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
	BaseUrl string
	Path    string
	PAT     string
}

func New(config Config) LinkValidador {
	processors := make([]LinkProcessor, 0)
	processors = append(processors, intern.New(config.BaseUrl, config.PAT))
	processors = append(processors, local.New())
	processors = append(processors, external.New(config.BaseUrl))
	return LinkValidador{processors}
}

func (v *LinkValidador) ProcessFiles(ctx context.Context, filesList []string, logger *zap.Logger) Stats {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	stats := Stats{}

	for _, fileName := range filesList {
		logger.Debug("processing file", zap.String("fileName", fileName))
		stats.Files++
		f, err := os.Open(fileName)
		if err != nil {
			logger.Error("Error opening file", zap.String("file", fileName), zap.Error(err))
			continue
		}
		defer f.Close()
		lines := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			links := v.processLine(line)
			for link, processor := range links {
				err := processor.Process(ctx, link, logger)
				stats.Links++
				if err != nil {
					var notFound errs.NotFoundError
					if errors.As(err, &notFound) {
						logger.Warn("link not found", zap.String("link", notFound.Error()), zap.String("filename", fileName), zap.Int("line", lines))
						stats.NotFound++
					} else {
						stats.Errors++
						logger.Warn("error validating link", zap.String("link", link), zap.Error(err))
					}
				}
			}
			lines++
		}
		stats.Lines = stats.Lines + lines
		logger.Debug("Processed: ", zap.Int("lines", stats.Lines), zap.String("fileName", fileName))
	}
	return stats
}

func (v *LinkValidador) GetFiles(root string, masks []string) ([]string, error) {
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
