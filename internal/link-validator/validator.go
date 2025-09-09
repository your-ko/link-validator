package link_validator

import (
	"bufio"
	"context"
	"go.uber.org/zap"
	"io/fs"
	"link-validator/pkg/git"
	"link-validator/pkg/http"
	"link-validator/pkg/local"
	"os"
	"path/filepath"
	"regexp"
)

type LinkProcessor interface {
	// Process expects to actually process the received text from slack from the given user
	// TODO: return stat of processed links (good/errored, error)
	Process(ctx context.Context, url string, logger *zap.Logger) error

	Regex() *regexp.Regexp
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
	processors = append(processors, git.New(config.BaseUrl, config.PAT))
	processors = append(processors, local.New(config.Path))
	processors = append(processors, http.New(config.BaseUrl))
	return LinkValidador{processors}
}

// TODO: return stats? So I can do a summary after the file is processed?
func (v *LinkValidador) ProcessFiles(filesList []string, logger *zap.Logger) (interface{}, error) {
	ctx := context.Background() // TODO: fix context

	for _, fileName := range filesList {
		logger.Debug("processing file", zap.String("fileName", fileName))
		f, err := os.Open(fileName)
		if err != nil {
			logger.Error("Error opening file", zap.String("file", fileName), zap.Error(err))
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			links := v.processLine(line)
			for link, processor := range links {
				err := processor.Process(ctx, link, logger)
				if err != nil {
					// TODO
					return nil, err
				}
			}

			//if httpProcessor.Regex().MatchString(line) {
			//	err = httpProcessor.Process(line, fileName, logger)
			//	if err != nil {
			//		statusCodeError := &http.StatusCodeError{}
			//		if errors.As(err, &statusCodeError) {
			//			logger.Error("can't read the link", zap.String("fileName", fileName), zap.Int("line", lineNum), zap.Int("statusCode", statusCodeError.StatusCode), zap.String("link", statusCodeError.Link))
			//		} else {
			//			logger.Error("error processing the link", zap.Error(err))
			//		}
			//	}
			//}
		}
		logger.Debug("Processed: ", zap.Int("lines", lineNum), zap.String("fileName", fileName))

	}
	return nil, nil
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
		parts := p.Regex().FindAllString(line, -1)
		for _, part := range parts {
			found[part] = p
		}
	}
	return found
}
