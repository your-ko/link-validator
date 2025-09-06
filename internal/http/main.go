package http

import (
	"bytes"
	"context"
	"go.uber.org/zap"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ExternalHttpLinkProcessor struct {
	httpClient *http.Client
	urlRegex   *regexp.Regexp
	exclude    string
}

func New(exclude string) *ExternalHttpLinkProcessor {
	//sanitized := strings.ReplaceAll(strings.TrimSuffix(strings.ReplaceAll(exclude, "https://", ""), "/"), ".", "\\.")
	//regex := fmt.Sprintf("https:\\/\\/(?!%s)([^\\s\"']+)", sanitized)
	//urlRegex := regexp.MustCompile(regex)
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		// TODO: not sure whether it is a good idea
		CheckRedirect: checkRedirect,
		// Otherwise follow redirects by default
		// If you want to limit the number of redirects, set CheckRedirect
	}
	return &ExternalHttpLinkProcessor{
		httpClient: httpClient,
		urlRegex:   regexp.MustCompile(`https:\/\/[^\s"']+`),
		exclude:    exclude,
	}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

func (proc *ExternalHttpLinkProcessor) Process(ctx context.Context, url string, logger *zap.Logger) error {
	if !strings.Contains(url, proc.exclude) {
		// excluded url found, skip it
		return nil
	}
	req, err := http.NewRequest("GET", url, bytes.NewBuffer(nil))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/html")

	//proc.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
	//	for key, val := range via[0].Header {
	//		req.Header[key] = val
	//	}
	//	return err
	//}
	r, err := proc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 200 && r.StatusCode <= 300 {
		// check just first 4 KB of the body
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err == nil && len(bodyBytes) > 0 {
			body := string(bodyBytes)
			//logger.Debug("body:", zap.String("body", body))	// TODO: remove
			if strings.Contains(body, "404") ||
				strings.Contains(body, "does not contain the path") ||
				strings.Contains(body, "not found") {
				//logger.Error("Broken HTTP link (soft 404)",
				//	zap.String("file", filePath),
				//	zap.Int("line", lineNum),
				//	zap.String("link", link),
				//	zap.String("body_snippet", body),
				//)
				return &StatusCodeError{404, url}
			} else {
				return nil
			}
		}
	}

	return &StatusCodeError{404, url}
}

func (proc *ExternalHttpLinkProcessor) Regex() *regexp.Regexp {
	return proc.urlRegex
}
