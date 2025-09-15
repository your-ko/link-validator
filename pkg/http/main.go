// Package http implements http links validation, i.e., any link starting with http(s),
// but not pointing to the GitHub, where the repository is (useful when run enterprise GitHub)
package http

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

type ExternalHttpLinkProcessor struct {
	httpClient *http.Client
	urlRegex   *regexp.Regexp
	exclude    string
}

func New(exclude string) *ExternalHttpLinkProcessor {
	exclude = strings.TrimPrefix(strings.TrimPrefix(exclude, "https://"), "http://")
	exclude = strings.TrimPrefix(strings.TrimSuffix(exclude, "/"), ".")
	httpClient := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: checkRedirect,
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
		// check just the first 4 KB of the body
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err == nil && len(bodyBytes) > 0 {
			body := string(bodyBytes)
			if strings.Contains(body, "404") ||
				strings.Contains(body, "does not contain the path") ||
				strings.Contains(body, "not found") {

				return errs.NewNotFound(url)
			} else {
				return nil
			}
		}
	}

	return errs.NewNotFound(url)
}

func (proc *ExternalHttpLinkProcessor) Regex() *regexp.Regexp {
	return proc.urlRegex
}

func (proc *ExternalHttpLinkProcessor) ExtractLinks(line string) []string {
	parts := proc.Regex().FindAllString(line, -1)
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
