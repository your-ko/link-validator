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
	processors    []LinkProcessor
	fileProcessor FileProcessorFunc
}

func New(cfg *config.Config) LinkValidador {
	processors := make([]LinkProcessor, 0)
	gh, err := github.New(cfg.CorpGitHubUrl, cfg.CorpPAT, cfg.PAT, cfg.Timeout, logger)
	if err != nil {
		logger.Error("can't instantiate GitHub link validator", zap.Error(err))
	}
	processors = append(processors, gh)
	processors = append(processors, local_path.New(logger))
	processors = append(processors, http.New(cfg.Timeout, cfg.IgnoredDomains, logger))

	if len(cfg.Files) != 0 {
		return LinkValidador{processors, includeFilesPipeline(cfg)}
	}
	return LinkValidador{processors, walkFilesPipeline(cfg)}
}

func includeFilesPipeline(cfg *config.Config) FileProcessorFunc {
	return ProcessFilesPipeline(
		IncludeExplicitFilesProcessor(cfg.Files),
		DeDupFilesProcessor(),
		ExcludePathsProcessor(cfg.Exclude),
		FilterByMaskProcessor(cfg.FileMasks),
	)
}

func walkFilesPipeline(cfg *config.Config) FileProcessorFunc {
	return ProcessFilesPipeline(
		WalkDirectoryProcessor(cfg),
		DeDupFilesProcessor(),
		ExcludePathsProcessor(cfg.Exclude),
		FilterByMaskProcessor(cfg.FileMasks),
	)
}

func (v *LinkValidador) ProcessFiles(ctx context.Context, filesList []string) Stats {
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
					logger.Warn("link not found", zap.String("link", link), zap.String("error", err.Error()), zap.String("filename", fileName), zap.Int("line", lines))
					stats.NotFoundLinks++
				} else if errors.Is(err, errs.ErrEmptyBody) {
					logger.Warn("link not found", zap.String("link", link), zap.String("error", err.Error()), zap.String("filename", fileName), zap.Int("line", lines))
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
	return v.fileProcessor([]string{})
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

// FileProcessorFunc is a function that processes a list of files
type FileProcessorFunc func(files []string) ([]string, error)

// =============================================================================
// FILE PROCESSORS
// =============================================================================

// WalkDirectoryProcessor returns a processor that walks a directory and finds files matching masks
func WalkDirectoryProcessor(cfg *config.Config) FileProcessorFunc {
	return func(files []string) ([]string, error) {
		// If explicit files are provided, don't walk directory
		if len(cfg.Files) > 0 {
			return files, nil
		}

		var result []string
		err := filepath.WalkDir(cfg.LookupPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Just skip files/dirs we can't read
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if matchesFileMask(d.Name(), cfg.FileMasks) {
				result = append(result, path)
			}
			return nil
		})
		return result, err
	}
}

// FilterByMaskProcessor returns a processor that filters files by mask patterns
func FilterByMaskProcessor(masks []string) FileProcessorFunc {
	return func(files []string) ([]string, error) {
		if len(files) == 0 || len(masks) == 0 {
			return files, nil
		}

		var result []string
		for _, fileName := range files {
			// Extract just the filename part from the full path for matching
			baseName := filepath.Base(fileName)
			for _, mask := range masks {
				match, err := filepath.Match(mask, baseName)
				if err != nil {
					return nil, err
				}
				if match {
					result = append(result, fileName)
					break // Found a match, no need to check other masks for this file
				}
			}
		}
		return result, nil
	}
}

// IncludeExplicitFilesProcessor returns a processor that includes explicit files when input is empty
func IncludeExplicitFilesProcessor(explicitFiles []string) FileProcessorFunc {
	return func(files []string) ([]string, error) {
		return explicitFiles, nil
	}
}

// DeDupFilesProcessor removes file duplicates
func DeDupFilesProcessor() FileProcessorFunc {
	return func(files []string) ([]string, error) {
		accu := make(map[string]bool)
		for _, fileName := range files {
			accu[fileName] = true
		}
		res := make([]string, 0, len(accu))
		for k := range accu {
			res = append(res, k)
		}
		return res, nil
	}
}

// ExcludePathsProcessor returns a processor that excludes specific paths
func ExcludePathsProcessor(exclude []string) FileProcessorFunc {
	return func(files []string) ([]string, error) {
		if len(exclude) == 0 {
			return files, nil
		}
		var result []string
		for _, fileName := range files {
			needToExclude := false
			for _, ex := range exclude {
				if strings.HasPrefix(fileName, ex) {
					needToExclude = true
					break
				}
			}
			if !needToExclude {
				result = append(result, fileName)
			}
		}
		return result, nil
	}
}

// ProcessFilesPipeline composes multiple processors into a pipeline
func ProcessFilesPipeline(processors ...FileProcessorFunc) FileProcessorFunc {
	return func(files []string) ([]string, error) {
		result := files
		for _, processor := range processors {
			var err error
			result, err = processor(result)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
}

// subtraction subtracts the right slice from the left slice
// so the result will contain elements of left slice that are not present in the right slice
func subtraction(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return left
	}
	accu := make(map[string]bool, len(left))
	for _, l := range left {
		accu[l] = true
	}
	for _, r := range right {
		delete(accu, r)
	}
	result := make([]string, 0, len(accu))
	for k := range accu {
		result = append(result, k)
	}
	return result
}
