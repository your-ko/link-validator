package link_validator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"link-validator/pkg/config"
	"link-validator/pkg/dd"
	"link-validator/pkg/errs"
	"link-validator/pkg/github"
	"link-validator/pkg/http"
	"link-validator/pkg/local-path"
	"link-validator/pkg/vault"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type LinkProcessor interface {
	Process(ctx context.Context, url string, testFileName string) error

	ExtractLinks(line string) []string
}

type HttpValidatorExcluder interface {
	// Excludes returns true if the url should be ignored by http validator as it is validated by another validator
	Excludes(url string) bool
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

func New(cfg *config.Config) (*LinkValidador, error) {
	processors := make([]LinkProcessor, 0)
	httpExcluders := make([]HttpValidatorExcluder, 0)
	ghValidator, err := github.New(cfg.CorpGitHubUrl, cfg.CorpPAT, cfg.PAT, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("can't instantiate GitHub link validator: %w", err)
	}
	httpExcluders = append(httpExcluders, ghValidator)
	processors = append(processors, ghValidator)
	ddValidator, err := dd.New(cfg)
	if err != nil {
		slog.Info("skip DataDog validator initialisation, DD_API_KEY/DD_APP_KEY are not set")
	} else {
		processors = append(processors, ddValidator)
		httpExcluders = append(httpExcluders, ddValidator)
	}
	vaultProcessor, err := vault.New(cfg.Vaults, cfg.Timeout)
	if err != nil {
		slog.With("error", err).Info("skip Vault validator initialisation due to %s")
	} else {
		processors = append(processors, vaultProcessor)
		httpExcluders = append(httpExcluders, vaultProcessor)
	}

	processors = append(processors, local_path.New())

	// Create exclusion function for HTTP processor
	// This function checks if any other processor can handle the URL
	excluder := func(url string) bool {
		for _, excluder := range httpExcluders {
			if excluder.Excludes(url) {
				return true
			}
		}
		return false
	}

	processors = append(processors, http.New(cfg.Timeout, cfg.IgnoredDomains, excluder))

	if len(cfg.Files) != 0 {
		return &LinkValidador{processors, includeFilesPipeline(cfg)}, nil
	}
	return &LinkValidador{processors, walkFilesPipeline(cfg)}, nil
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
		slog.Debug("Processing file", slog.String("fileName", fileName))
		stats.Files++
		f, err := os.Open(fileName)
		if err != nil {
			slog.With("error", err).Error("Error opening file", slog.String("file", fileName))
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
				err = processor.Process(ctx, link, fileName)
				linksFound++
				if err == nil {
					slog.Debug("link validation successful", slog.String("link", link), slog.String("filename", fileName), slog.Int("line", lines))
					continue
				}

				if errors.Is(err, errs.ErrNotFound) {
					slog.Warn("not found", slog.String("link", link), slog.String("error", err.Error()), slog.String("filename", fileName), slog.Int("line", lines+1))
					stats.NotFoundLinks++
				} else if errors.Is(err, errs.ErrEmptyBody) {
					slog.Warn("not found", slog.String("link", link), slog.String("error", err.Error()), slog.String("filename", fileName), slog.Int("line", lines+1))
					stats.NotFoundLinks++
				} else {
					stats.Errors++
					slog.With("error", err).Error("error validating link", slog.String("link", link), slog.String("filename", fileName), slog.Int("line", lines+1))
				}
			}
			lines++
		}
		if err := scanner.Err(); err != nil {
			slog.Warn("scan failed", slog.String("file", fileName), "err", err)
		}
		// Close file next iteration
		if err := f.Close(); err != nil {
			slog.Warn("close failed", slog.String("file", fileName), "err", err)
		}
		stats.Lines = stats.Lines + lines
		stats.TotalLinks = stats.TotalLinks + linksFound

		slog.Info("Processed", slog.Int("lines", lines), slog.Int("links", linksFound), slog.String("fileName", fileName))
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
			if _, exist := found[link]; exist {
				slog.Warn("two processors compete for the link", slog.String("link", link))
			}
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
