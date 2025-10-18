// Package external implements http links validation, i.e., any link starting with http(s)
// also covers gihub non-repository links, such as api.github.com or https://github.com/your-ko/link-validator/pull
package external

import (
	"bytes"
	"context"
	"go.uber.org/zap"
	"io"
	"link-validator/pkg/errs"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type LinkProcessor struct {
	httpClient  *http.Client
	urlRegex    *regexp.Regexp
	githubRegex *regexp.Regexp
}

func New(timeout time.Duration) *LinkProcessor {
	httpClient := &http.Client{
		Timeout:       timeout,
		CheckRedirect: checkRedirect,
	}
	urlRegex := regexp.MustCompile(`https:\/\/[^\s"'()\[\]]+`)
	// ghRegex is identical to the intern.repoRegex, but it is used in inverse way
	ghRegex := regexp.MustCompile(`(?i)https://github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)(?:/[^\s"'()<>\[\]{}?#]+)*(?:#[^\s"'()<>\[\]{}]+)?`)

	return &LinkProcessor{
		httpClient:  httpClient,
		urlRegex:    urlRegex,
		githubRegex: ghRegex,
	}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, logger *zap.Logger) error {
	logger.Debug("Validating external url", zap.String("url", url))

	req, err := http.NewRequestWithContext(ctx, "GET", url, bytes.NewBuffer(nil))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "link-validator/1.0 (+https://github.com/your-ko/link-validator)")

	r, err := proc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		logger.Debug("", zap.Int("statusCode", r.StatusCode))
		return errs.NewNotFound(url)
	}

	// check just the first 4 KB of the body
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10240))
	if err != nil {
		// we can't read body, something is off
		return err
	}
	if len(bodyBytes) == 0 {
		// body is empty, doesn't count as a healthy URL
		return errs.NewEmptyBody(url)
	}

	body := strings.ToLower(string(bodyBytes))
	if strings.Contains(body, "404") ||
		strings.Contains(body, "does not contain the path") ||
		strings.Contains(body, "not found") {

		return errs.NewNotFound(url)
	} else {
		return nil
	}
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := proc.urlRegex.FindAllString(line, -1)
	urls := make([]string, 0, len(parts))

	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue // skip malformed
		}
		if proc.githubRegex.MatchString(raw) {
			continue // skip the majority of GitHub urls
		}

		urls = append(urls, raw)
	}
	return urls
}
