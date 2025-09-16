// Package 'local' implements local links validation
// Local links are the links found in the given repository, which point to files in the same repository.
// Example: [README](../../README.md)
// http(s):// links are not processes

package local

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"link-validator/pkg/errs"
	"os"
	"regexp"
)

type LinkProcessor struct {
	fileRegex *regexp.Regexp
	path      string
}

func New() *LinkProcessor {
	localTarget := `(?:` +
		`(?:\./|\.\./)+(?:[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*)?` + // ./... or ../... any depth
		`|` +
		`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*` + // bare filename / relative path
		`)` +
		`(?:#[^)\s]*)?` // optional fragment

	regexp := regexp.MustCompile(`\[[^\]]*\]\((` + localTarget + `)\)`)

	return &LinkProcessor{
		fileRegex: regexp,
	}
}

func (proc *LinkProcessor) Process(_ context.Context, url string, logger *zap.Logger) error {
	parts := proc.fileRegex.FindAllStringSubmatch(url, -1)
	if len(parts) != 1 && len(parts[0]) != 2 {
		return fmt.Errorf("incorrect md syntax: %s", url)
	}
	fileName := fmt.Sprintf("%s/%s", proc.path, parts[0][1])
	logger.Debug("validating local path", zap.String("filename", fileName))
	_, err := os.ReadFile(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errs.NewNotFound(fileName)
		}
		return err
	}
	return nil
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	matches := proc.fileRegex.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil
	}
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		// m[0] = full token "[txt](target)", m[1] = captured target
		if len(m) > 1 && m[1] != "" {
			urls = append(urls, m[1])
		}
	}
	return urls
}
