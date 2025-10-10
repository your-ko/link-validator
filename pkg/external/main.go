// Package external implements http links validation, i.e., any link starting with http(s)
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

type HttpLinkProcessor struct {
	httpClient *http.Client
	urlRegex   *regexp.Regexp
	exclude    string
}

func New(exclude string) *HttpLinkProcessor {
	exclude = strings.TrimPrefix(strings.TrimPrefix(exclude, "https://"), "http://")
	exclude = strings.TrimPrefix(strings.TrimSuffix(exclude, "/"), ".")
	httpClient := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: checkRedirect,
	}
	urlRegex := regexp.MustCompile(`https:\/\/[^\s"'()\[\]]+`)

	return &HttpLinkProcessor{
		httpClient: httpClient,
		urlRegex:   urlRegex,
		exclude:    exclude,
	}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

func (proc *HttpLinkProcessor) Process(_ context.Context, url string, logger *zap.Logger) error {
	logger.Debug("Validating external url", zap.String("url", url))

	req, err := http.NewRequest("GET", url, bytes.NewBuffer(nil))
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
		return errs.NewNotFound(url)
	}

	// check just the first 4 KB of the body
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 4096))
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

func (proc *HttpLinkProcessor) ExtractLinks(line string) []string {
	parts := proc.urlRegex.FindAllString(line, -1)
	urls := make([]string, 0, len(parts))

	if proc.exclude == "" {
		// nothing to exclude; return all matches quickly
		return append(urls, parts...)
	}

	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue // skip malformed
		}
		host := strings.ToLower(u.Hostname()) // strips port, handles IPv6 brackets

		// Exclude exact domain or any subdomain.
		if host == proc.exclude || strings.HasSuffix(host, "."+proc.exclude) {
			continue
		}

		urls = append(urls, raw)
	}
	return urls
}
