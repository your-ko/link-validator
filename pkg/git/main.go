// Package 'git' implements git links validation
// GitHub links are the links that point to files in other GitHub repositories
// These links can be considered as internal (in contrast to external in `http` package and `local` in `local` package
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package git

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
	"link-validator/pkg/errs"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type InternalLinkProcessor struct {
	baseUrl  string
	client   *github.Client
	urlRegex *regexp.Regexp
}

func New(baseUrl, pat string) *InternalLinkProcessor {
	client, err := github.NewClient(nil).WithEnterpriseURLs(
		baseUrl,
		strings.ReplaceAll(baseUrl, "https://", "https://uploads."),
	)
	if err != nil {
		panic(fmt.Sprintf("can't create GitHub Processor: %s", err))
	}
	client = client.WithAuthToken(pat)

	// Derive the bare host from baseUrl, e.g. "github.mycorp.com"

	u, err := url.Parse(baseUrl)
	if err != nil || u.Hostname() == "" {
		panic(fmt.Sprintf("invalid baseUrl: %q", baseUrl))
	}
	host := u.Hostname()

	// Escape dots for regex, build a subdomain-capable host: (?:[A-Za-z0-9-]+\.)*github\.mycorp\.com
	escHost := regexp.QuoteMeta(host)
	hostPattern := fmt.Sprintf(`(?:[A-Za-z0-9-]+\.)*%s`, escHost)

	// Keep  path structure (org/repo/(blob|tree|raw)/branch/optional path ... optional #fragment)
	// Allow optional query/fragment tails in the last groups (your original already allowed #...).
	pattern := fmt.Sprintf(
		`https://%s/([^/\s"']+)/([^/\s"']+)/(blob|tree|raw)/([^/\s"']+)(?:/([^\#\s\)\]]*))?(#[^\s\)\]]+)?`,
		hostPattern,
	)

	urlRegex := regexp.MustCompile(pattern)

	return &InternalLinkProcessor{
		baseUrl:  baseUrl,
		client:   client,
		urlRegex: urlRegex,
	}
}

func (proc *InternalLinkProcessor) Process(ctx context.Context, url string, logger *zap.Logger) error {
	match := proc.urlRegex.FindStringSubmatch(url)
	if len(match) == 0 {
		return fmt.Errorf("invalid or unsupported GitHub URL: %s", url)
	}
	owner, repo, _, branch, path, anchor := match[1], match[2], match[3], match[4], strings.TrimPrefix(match[5], "/"), match[6]
	logger.Debug("Validating GutHub url", zap.String("url", url))

	contents, _, _, err := proc.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		var ghError *github.ErrorResponse
		if errors.As(err, &ghError) {
			if ghError.Response.StatusCode == http.StatusNotFound {
				return errs.NotFound
			}
		}
		// some other error
		return err
	}
	logger.Debug("Validating anchor in GitHub URL", zap.String("link", url), zap.String("anchor", anchor))
	if len(anchor) != 0 {
		content, err := contents.GetContent()
		if err != nil {
			return err
		}
		if !strings.Contains(content, anchor) {
			logger.Info("url exists but doesn't have an anchor", zap.String("link", url), zap.String("anchor", anchor))
			return errs.NewNotFound(url)
		} else {
			// url with the anchor are correct
			return nil
		}
	}
	// url exists
	return nil
}

func (proc *InternalLinkProcessor) Regex() *regexp.Regexp {
	return proc.urlRegex
}

func (proc *InternalLinkProcessor) ExtractLinks(line string) []string {
	parts := proc.Regex().FindAllString(line, -1)
	if len(parts) == 0 {
		return nil
	}

	// Parse base hostname once
	base, err := url.Parse(proc.baseUrl)
	if err != nil || base.Hostname() == "" {
		return nil
	}
	baseHost := strings.ToLower(base.Hostname())

	urls := make([]string, 0, len(parts))
	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		h := strings.ToLower(u.Hostname())

		// Keep only internal: exact host or any subdomain of it
		if h == baseHost || strings.HasSuffix(h, "."+baseHost) {
			urls = append(urls, raw)
		}
	}
	return urls
}
