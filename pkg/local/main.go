// Package 'local' implements local links validation
// Local links are the links found in the given repository, which point to files in the same repository.
// Example: [README](../../README.md)

package local

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"os"
	"regexp"
)

type LinkProcessor struct {
	fileRegex *regexp.Regexp
	path      string
}

func New(path string) *LinkProcessor {
	return &LinkProcessor{
		fileRegex: regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`),
		path:      path,
	}
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, logger *zap.Logger) error {
	parts := proc.Regex().FindAllStringSubmatch(url, -1)
	if len(parts) != 1 && len(parts[0]) != 2 {
		return fmt.Errorf("incorrect md syntax: %s", url)
	}
	fileName := fmt.Sprintf("%s/%s", proc.path, parts[0][1])
	logger.Debug("validating local url", zap.String("filename", fileName))
	_, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}
	return nil
}

func (proc *LinkProcessor) Regex() *regexp.Regexp {
	return proc.fileRegex
}
