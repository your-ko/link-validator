package link_validator

import (
	"bufio"
	"context"
	"errors"
	"io/fs"
	"link-validator/pkg/config"
	"link-validator/pkg/errs"
	"link-validator/pkg/github"
	"link-validator/pkg/http"
	"link-validator/pkg/local-path"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type LinkProcessor interface {
	Process(ctx context.Context, url string, testFileName string) error

	ExtractLinks(line string) []string
}

type Stats struct {
	Lines         int
	TotalLinks    int
	Errors        int
	NotFoundLinks int
	Files         int
}

type LinkValidador struct {
	processors     []LinkProcessor
	fileProcessors []fileProcessor
}

func New(cfg *config.Config, logger *zap.Logger) LinkValidador {
	processors := make([]LinkProcessor, 0)
	gh, err := github.New(cfg.CorpGitHubUrl, cfg.CorpPAT, cfg.PAT, cfg.Timeout, logger)
	if err != nil {
		logger.Error("can't instantiate GitHub link validator", zap.Error(err))
	}
	processors = append(processors, gh)
	processors = append(processors, local_path.New(logger))
	processors = append(processors, http.New(cfg.Timeout, cfg.IgnoredDomains, logger))
	return LinkValidador{processors, getFileProcessors(cfg)}
}

func getFileProcessors(cfg *config.Config) []fileProcessor {
	fileProcessors := make([]fileProcessor, 0)
	fileProcessors = append(fileProcessors, newWalkerFilesProcessor(cfg))
	fileProcessors = append(fileProcessors, newIncludedFilesProcessor(cfg))
	fileProcessors = append(fileProcessors, newFileMaskProcessor(cfg))
	fileProcessors = append(fileProcessors, newExcludedPathProcessor(cfg))
	return fileProcessors
}

func (v *LinkValidador) ProcessFiles(ctx context.Context, filesList []string, logger *zap.Logger) Stats {
	stats := Stats{}

	for _, fileName := range filesList {
		logger.Debug("Processing file:", zap.String("fileName", fileName))
		stats.Files++
		f, err := os.Open(fileName)
		if err != nil {
			logger.Error("Error opening file", zap.String("file", fileName), zap.Error(err))
			continue
		}

		lines := 0
		linksFound := 0
		scanner := bufio.NewScanner(f)
		codeSnippet := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "```") {
				codeSnippet = !codeSnippet
			}
			if codeSnippet {
				lines++
				continue
			}
			links := v.processLine(line)
			for link, processor := range links {
				err := processor.Process(ctx, link, fileName)
				linksFound++
				if err == nil {
					logger.Debug("link validation successful", zap.String("link", link), zap.String("filename", fileName), zap.Int("line", lines))
					continue
				}

				if errors.Is(err, errs.ErrNotFound) {
					logger.Warn("link not found", zap.String("error", err.Error()), zap.String("filename", fileName), zap.Int("line", lines))
					stats.NotFoundLinks++
				} else if errors.Is(err, errs.ErrEmptyBody) {
					logger.Warn("link not found", zap.String("error", err.Error()), zap.String("filename", fileName), zap.Int("line", lines))
					stats.NotFoundLinks++
				} else {
					stats.Errors++
					logger.Warn("error validating link", zap.String("link", link), zap.String("filename", fileName), zap.Int("line", lines), zap.Error(err))
				}
			}
			lines++
		}
		if err := scanner.Err(); err != nil {
			logger.Warn("scan failed", zap.String("file", fileName), zap.Error(err))
		}
		// Close file next iteration
		if err := f.Close(); err != nil {
			logger.Warn("close failed", zap.String("file", fileName), zap.Error(err))
		}
		stats.Lines = stats.Lines + lines
		stats.TotalLinks = stats.TotalLinks + linksFound

		if logger.Core().Enabled(zap.DebugLevel) {
			logger.Debug("Processed: ", zap.Int("lines", lines), zap.Int("links", linksFound), zap.String("fileName", fileName))
		} else {
			logger.Info("Processed: ", zap.String("fileName", fileName))
		}
	}
	return stats
}

// matchesFileMask checks if a filename matches any of the provided file masks
func matchesFileMask(filename string, masks []string) bool {
	// Extract just the filename part from the full path
	baseName := filepath.Base(filename)
	for _, mask := range masks {
		matched, err := filepath.Match(mask, baseName)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// GetFiles returns a list of files to process based on configuration
func (v *LinkValidador) GetFiles() ([]string, error) {
	files := make([]string, 0)
	var err error
	for _, proc := range v.fileProcessors {
		files, err = proc.getFiles(files)
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func (v *LinkValidador) processLine(line string) map[string]LinkProcessor {
	found := make(map[string]LinkProcessor)
	for _, p := range v.processors {
		links := p.ExtractLinks(line)
		for _, link := range links {
			found[link] = p
		}
	}
	return found
}

type fileProcessor interface {
	getFiles([]string) ([]string, error)
}

type fileMaskProcessor struct {
	config *config.Config
}
type includedFilesProcessor struct {
	config *config.Config
}
type walkerFilesProcessor struct {
	config *config.Config
}
type excludedPathProcessor struct {
	config *config.Config
}

func newWalkerFilesProcessor(cfg *config.Config) *walkerFilesProcessor {
	return &walkerFilesProcessor{config: cfg}
}
func newFileMaskProcessor(cfg *config.Config) *fileMaskProcessor {
	return &fileMaskProcessor{config: cfg}
}
func newIncludedFilesProcessor(cfg *config.Config) *includedFilesProcessor {
	return &includedFilesProcessor{config: cfg}
}
func newExcludedPathProcessor(cfg *config.Config) *excludedPathProcessor {
	return &excludedPathProcessor{config: cfg}
}

func (f *walkerFilesProcessor) getFiles(_ []string) ([]string, error) {
	var result []string
	if len(f.config.Files) != 0 {
		return result, nil
	}
	err := filepath.WalkDir(f.config.LookupPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Just skip files/dirs we can't read
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if matchesFileMask(d.Name(), f.config.FileMasks) {
			result = append(result, path)
		}
		return nil
	})

	return result, err
}

func (f *fileMaskProcessor) getFiles(input []string) ([]string, error) {
	if len(input) == 0 {
		return input, nil
	}
	res := make([]string, 0)
	for _, fileName := range input {
		// Extract just the filename part from the full path for matching
		baseName := filepath.Base(fileName)
		for _, mask := range f.config.FileMasks {
			match, err := filepath.Match(mask, baseName)
			if err != nil {
				return nil, err
			}
			if match {
				res = append(res, fileName)
				break // Found a match, no need to check other masks for this file
			}
		}
	}
	return res, nil
}

func (f *includedFilesProcessor) getFiles(input []string) ([]string, error) {
	if len(input) == 0 {
		return f.config.Files, nil
	} else {
		return input, nil
	}
}

func (f *excludedPathProcessor) getFiles(input []string) ([]string, error) {
	if len(f.config.ExcludePath) == 0 {
		return input, nil
	}
	return subtraction(input, f.config.ExcludePath), nil
}

func subtraction(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return left
	}
	accu := make(map[string]bool, len(left))
	for _, l := range left {
		accu[l] = true
	}
	for _, r := range right {
		if _, ok := accu[r]; ok {
			delete(accu, r)
		}
	}
	result := make([]string, len(accu))
	for k := range accu {
		result = append(result, k)
	}
	return result
}

func interception(left, right []string) []string {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	accu := make(map[string]bool, len(left))
	result := make([]string, 0)
	for _, l := range left {
		accu[l] = true
	}
	for _, r := range right {
		if _, ok := accu[r]; ok {
			result = append(result, r)
		}
	}
	return result
}
