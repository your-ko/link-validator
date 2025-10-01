// Package 'local' implements local links validation
// Local links are the links found in the given repository, which point to files in the same repository.
// Example: [README](../../README.md)
// http(s):// links are not processes

package local

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"link-validator/pkg/errs"
	"os"
	"regexp"
)

type LinkProcessor struct {
	fileRegex *regexp.Regexp
}

func New() *LinkProcessor {
	localTarget := `(?:` +
		`(?:\./|\.\./)+(?:[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*)?` + // ./... or ../... any depth
		`|` +
		`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*` + // bare filename / relative path
		`)` +
		`(?:#[^)\s]*)?` // optional fragment

	regex := regexp.MustCompile(`\[[^\]]*\]\((` + localTarget + `)\)`)

	return &LinkProcessor{
		fileRegex: regex,
	}
}

func (proc *LinkProcessor) Process(_ context.Context, path string, logger *zap.Logger) error {
	logger.Debug("validating local url", zap.String("filename", path))
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errs.NewNotFound(path)
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
