// Package http implements https links validation, i.e., any link starting with http(s), which are not GitHub links.
// also covers GitHub non-repository links, such as api.github.com
package http

import (
	"bytes"
	"context"
	"io"
	"link-validator/pkg/errs"
	"link-validator/pkg/regex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LinkProcessor struct {
	httpClient     *http.Client
	ignoredDomains []string
}

func New(timeout time.Duration, ignoredDomains []string) *LinkProcessor {
	httpClient := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			slog.Debug("redirecting", slog.String("to", req.URL.String()), slog.Int("hops", len(via)))
			redirectLimit := 3
			if len(via) > redirectLimit {
				slog.Warn("too many redirects", slog.Int("redirect limit", redirectLimit), slog.String("url", via[0].URL.String()))
			}
			for k, vs := range via[0].Header {
				if req.Header.Get(k) == "" {
					for _, v := range vs {
						req.Header.Add(k, v)
					}
				}
			}
			return nil
		},
	}

	return &LinkProcessor{
		httpClient:     httpClient,
		ignoredDomains: ignoredDomains,
	}
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, _ string) error {
	slog.Debug("http: starting validation", slog.String("url", url))

	url = strings.TrimSuffix(url, "/")
	req, err := http.NewRequestWithContext(ctx, "GET", url, bytes.NewBuffer(nil))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "link-validator/1.0 (+https://github.com/your-ko/link-validator)")

	resp, err := proc.httpClient.Do(req)
	if err != nil {
		return err
	}

	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		// we can proceed without authentication, so we don't know whether the url is alive.
		// maybe in the future this will be improved
		slog.Info("http: requires auth", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return nil
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		slog.Debug("http: not found", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return errs.NewNotFound(url)
	case resp.StatusCode == 429:
		slog.Info("http: probably rate limit", slog.String("ra", resp.Header.Get("Retry-After")), slog.String("url", url))
		return nil
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		slog.Info("http: ignoring the url validation due to problems on the remote server", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return nil
	case 200 <= resp.StatusCode && resp.StatusCode <= 299:
		// check just the first 1 KB of the body
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				slog.With("error", err).Warn("http: can't close response body", slog.String("url", url))
			}
		}(resp.Body)
		bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			// we can't read body, something is off
			return err
		}

		if len(bodyBytes) == 0 {
			// body is empty, doesn't count as a healthy URL
			return errs.NewEmptyBody(url)
		}

		return nil
	default:
		slog.Warn("http: unexpected status", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return nil
	}
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := regex.Url.FindAllString(line, -1)
	urls := make([]string, 0, len(parts))

	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			slog.Debug("http: url seems to be malformed", slog.String("url", raw))
			continue // skip malformed
		}
		if strings.Contains(raw, "localhost") {
			slog.Debug("http: localhost is ignored", slog.String("url", raw))
			continue // no need to validate localhost
		}
		if strings.ContainsAny(raw, "[]{}()") {
			slog.Debug("http: url seems to be templated", slog.String("url", raw))
			continue
		}
		if proc.urlShouldBeIgnored(raw) {
			slog.Debug("http: url should be ignored", slog.String("url", raw))
			continue
		}
		if regex.GitHub.MatchString(raw) && !regex.GitHubExcluded.MatchString(raw) {
			continue // skip GitHub urls except for non-API ones
		}
		if regex.DataDog.MatchString(raw) {
			continue // skip DataDog urls
		}

		urls = append(urls, raw)
	}
	return urls
}

func (proc *LinkProcessor) urlShouldBeIgnored(url string) bool {
	for _, d := range proc.ignoredDomains {
		if strings.HasPrefix(url, d) {
			return true
		}
	}
	return false
}
