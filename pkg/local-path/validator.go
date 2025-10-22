// Package local-path implements local links validation
// Local links are the links found in the given repository, which point to files in the same repository.
// Example: [README](../../README.md)

package local_path

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"link-validator/pkg/errs"
	"os"
	"regexp"
	"strings"
)

type LinkProcessor struct {
	fileRegex *regexp.Regexp
	logger    *zap.Logger
}

func New(logger *zap.Logger) *LinkProcessor {
	localTarget := `(?:` +
		`(?:\./|\.\./)+(?:[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*)?` + // ./... or ../... any depth
		`|` +
		`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*` + // bare filename / relative path
		`)` +
		`(?:#[^)\s]*)?` // optional fragment

	regex := regexp.MustCompile(`\[[^\]]*\]\((` + localTarget + `)\)`)

	return &LinkProcessor{
		fileRegex: regex,
		logger:    logger,
	}
}

func (proc *LinkProcessor) Process(_ context.Context, link string, testFileName string) error {
	proc.logger.Debug("validating local url", zap.String("filename", link))
	testFileNameSplit := strings.Split(testFileName, "/")
	testPath := strings.Join(testFileNameSplit[:len(testFileNameSplit)-1], "/")

	split := strings.Split(link, "#")
	fileName := split[0]
	if strings.HasPrefix(fileName, "./") {
		fileName = strings.Replace(fileName, "./", "", 1)
	}

	fileNameToTest := fmt.Sprintf("%s/%s", testPath, fileName)

	// validate link format
	//if len(split) > 2 {
	//	return errors.New("incorrect link. Contains more than one #")
	//}
	var header string
	if len(split) > 1 {
		if len(split[1]) == 0 {
			// case [text](../link#) is incorrect
			return errs.NewEmptyHeadingError(fmt.Sprintf("%s#", fileNameToTest))
		}
		header = split[1]
		//re := regexp.MustCompile(`^[a-z0-9]+$`) TODO: improve
		//if !re.MatchString(header) {
		//	return errors.New("incorrect link. Contains upper case, which is not allowed")
		//}
	}
	info, err := os.Stat(fileNameToTest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errs.NewNotFound(fileNameToTest)
		}
		return err
	}
	if info.IsDir() {
		if header != "" {
			return errs.NewHeadingLinkToDir(fmt.Sprintf("%s#%s", fileNameToTest, header))
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
