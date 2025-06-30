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
	_, err := os.ReadFile(fmt.Sprintf("%s/%s", proc.path, parts[0][1]))
	if err != nil {
		return err
	}
	return nil
}

func (proc *LinkProcessor) Regex() *regexp.Regexp {
	return proc.fileRegex
}
