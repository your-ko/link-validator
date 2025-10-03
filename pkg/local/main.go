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
	"strings"
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

func (proc *LinkProcessor) Process(_ context.Context, link string, logger *zap.Logger) error {
	logger.Debug("validating local url", zap.String("filename", link))
	split := strings.Split(link, "#")

	// validate link format
	//if len(split) > 2 {
	//	return errors.New("incorrect link. Contains more than one #")
	//}
	var header string
	if len(split) > 1 {
		if len(split[1]) == 0 {
			// case [text](../link#) is incorrect
			return errs.NewEmptyHeadingError(link)
		}
		header = split[1]
		//re := regexp.MustCompile(`^[a-z0-9]+$`) TODO: improve
		//if !re.MatchString(header) {
		//	return errors.New("incorrect link. Contains upper case, which is not allowed")
		//}
	}
	fileName := split[0]

	info, err := os.Stat(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errs.NewNotFound(fileName)
		}
		return err
	}
	if info.IsDir() {
		if header != "" {
			return errs.NewHeadingLinkToDir(link)
		}
		return nil
	}
	return nil

	// TODO: heading validation will be done later
	//if header == "" {
	//	// we found the file, so everything is good
	//	return nil
	//}
	//
	//// if the link contains #, then we need to look inside the file
	//file, err := os.Open(fileName)
	//if err != nil {
	//	return err
	//}
	//defer file.Close()

	//re := regexp.MustCompile(fmt.Sprintf(`^#\s+%s`, regexp.QuoteMeta(header)))
	//scanner := bufio.NewScanner(file)
	//for scanner.Scan() {
	//	if re.MatchString(scanner.Text()) {
	//		return nil
	//	}
	//}
	//if err := scanner.Err(); err != nil {
	//	return err
	//}
	//return errs.NewNotFound(link)
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
