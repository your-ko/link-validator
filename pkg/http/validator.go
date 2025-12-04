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
				slog.Error("too many redirects", slog.Int("redirect limit", redirectLimit))
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
	slog.Debug("Validating external url", slog.String("url", url))

	if proc.urlShouldBeIgnored(url) {
		slog.Debug("url should be ignored", slog.String("url", url))
		return nil
	}

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
		slog.Info("requires auth", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return nil
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		slog.Debug("not found", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return errs.NewNotFound(url)
	case resp.StatusCode == 429:
		slog.Info("probably rate limit", slog.String("ra", resp.Header.Get("Retry-After")), slog.String("url", url))
		return nil
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		slog.Info("ignoring the url validation due to problems on the remote server", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return nil
	case 200 <= resp.StatusCode && resp.StatusCode <= 299:
		// check just the first 10 KB of the body
		bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10240))
		if err != nil {
			// we can't read body, something is off
			return err
		}
		err = resp.Body.Close()
		if err != nil {
			slog.Info("error closing body: %s", err)
		}

		if len(bodyBytes) == 0 {
			// body is empty, doesn't count as a healthy URL
			return errs.NewEmptyBody(url)
		}

		body := strings.ToLower(string(bodyBytes))
		if strings.Contains(body, "page not found") {
			// TODO: this needs to be improved
			return errs.NewNotFound(url)
		} else {
			return nil
		}
	default:
		slog.Warn("unexpected status", slog.Int("statusCode", resp.StatusCode), slog.String("url", url))
		return nil
	}
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := regex.Url.FindAllString(line, -1)
	urls := make([]string, 0, len(parts))

	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue // skip malformed
		}
		if strings.ContainsAny(raw, "[]{}()") {
			continue // seems it is the templated url
		}
		if regex.GitHub.MatchString(raw) {
			continue // skip GitHub urls
		}

		urls = append(urls, raw)
	}
	return urls
}

func (proc *LinkProcessor) urlShouldBeIgnored(url string) bool {
	for _, d := range proc.ignoredDomains {
		if strings.Contains(url, d) {
			return true
		}
	}
	return false
}
