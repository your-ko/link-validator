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

var urlRegex = regexp.MustCompile(`https:\/\/[^\s"'()\[\]]+`)

// ghRegex is identical to the gh.repoRegex, but it is used in inverse way
var ghRegex = regexp.MustCompile(`(?i)https://github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)(?:/[^\s"'()<>\[\]{}?#]+)*(?:#[^\s"'()<>\[\]{}]+)?`)

type LinkProcessor struct {
	httpClient *http.Client
	logger     *zap.Logger
}

func New(timeout time.Duration, logger *zap.Logger) *LinkProcessor {
	httpClient := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			logger.Debug("redirecting", zap.String("to", req.URL.String()), zap.Int("hops", len(via)))
			redirectLimit := 3
			if len(via) > redirectLimit {
				logger.Error("too many redirects", zap.Int("redirect limit", redirectLimit))
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
		httpClient: httpClient,
		logger:     logger,
	}
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, _ string) error {
	proc.logger.Debug("Validating external url", zap.String("url", url))

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
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		// we can proceed without authentication, so we don't know whether the url is alive.
		// maybe in the future this will be improved
		proc.logger.Info("requires auth", zap.Int("statusCode", resp.StatusCode), zap.String("url", url))
		return nil
	case resp.StatusCode == 404 || resp.StatusCode == 410:
		proc.logger.Debug("not found", zap.Int("statusCode", resp.StatusCode), zap.String("url", url))
		return errs.NewNotFound(url)
	case resp.StatusCode == 429:
		proc.logger.Info("probably rate limit", zap.String("ra", resp.Header.Get("Retry-After")), zap.String("url", url))
		return nil
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		proc.logger.Info("ignoring the url validation due to problems on the remote server", zap.Int("statusCode", resp.StatusCode), zap.String("url", url))
		return nil
	case 200 <= resp.StatusCode && resp.StatusCode <= 299:
		// check just the first 10 KB of the body
		bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10240))
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
	default:
		proc.logger.Warn("unexpected status", zap.Int("statusCode", resp.StatusCode), zap.String("url", url))
		return nil
	}
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := urlRegex.FindAllString(line, -1)
	urls := make([]string, 0, len(parts))

	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue // skip malformed
		}
		if ghRegex.MatchString(raw) {
			continue // skip the majority of GitHub urls
		}

		urls = append(urls, raw)
	}
	return urls
}
