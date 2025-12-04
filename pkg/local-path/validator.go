// Package local-path implements local links validation
// Local links are the links found in the given repository, which point to files in the same repository.
// Example: [README](../../README.md)

package local_path

import (
	"context"
	"errors"
	"fmt"
	"link-validator/pkg/errs"
	"link-validator/pkg/regex"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type LinkProcessor struct {
}

func New() *LinkProcessor {
	return &LinkProcessor{}
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	matches := regex.LocalPath.FindAllStringSubmatch(line, -1)
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

func (proc *LinkProcessor) Process(_ context.Context, link string, testFileName string) error {
	slog.Debug("validating local url", slog.String("filename", link))

	// Parse link into file path and optional header
	linkPath, header, err := proc.parseLink(link)
	if err != nil {
		return err
	}

	// Resolve the target file path relative to the test file
	targetPath := proc.resolveTargetPath(linkPath, testFileName)

	// Validate the target file exists and handle directory/header logic
	return proc.validateTarget(targetPath, header)
}

// parseLink separates the file path from the optional anchor fragment
func (proc *LinkProcessor) parseLink(link string) (path, anchor string, err error) {
	parts := strings.SplitN(link, "#", 2)
	path = parts[0]

	if len(parts) > 1 {
		if parts[1] == "" {
			return "", "", errs.NewEmptyAnchorError(fmt.Sprintf("%s#", path))
		}
		anchor = parts[1]
	}

	return path, anchor, nil
}

func (proc *LinkProcessor) resolveTargetPath(linkPath, testFileName string) string {
	// If linkPath is absolute, return as-is
	if filepath.IsAbs(linkPath) {
		return linkPath
	}

	testDir := filepath.Dir(testFileName)

	// Clean the link path and resolve it relative to the test file directory
	cleanPath := strings.TrimPrefix(linkPath, "./")
	targetPath := filepath.Join(testDir, cleanPath)

	// Preserve the ./ prefix in the result if it was originally present
	if strings.HasPrefix(linkPath, "./") {
		return filepath.Join(".", targetPath)
	}

	return targetPath
}

// validateTarget checks if the target exists and validates directory/header combinations
func (proc *LinkProcessor) validateTarget(targetPath, header string) error {
	info, err := os.Stat(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errs.NewNotFound(targetPath)
		}
		return err
	}

	// Directories with headers are invalid (can't link to a heading in a directory)
	if info.IsDir() && header != "" {
		return errs.NewAnchorLinkToDir(fmt.Sprintf("%s#%s", targetPath, header))
	}

	return nil
}
