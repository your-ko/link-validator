// Package 'internal' implements internal git links validation
// GitHub links are the links that point to files in other GitHub repositories within the same organisation
// These links can be considered as internal (in contrast to external package and `local`  package)
// Useful when run on the enterprise GitHub.
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package intern

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
	corpGitHubUrl string
	corpClient    *github.Client
	client        *github.Client
	urlRegex      *regexp.Regexp
}

func New(corpGitHubUrl, corpPat, pat string) *InternalLinkProcessor {
	// Derive the bare host from baseUrl, e.g. "github.mycorp.com"
	u, err := url.Parse(corpGitHubUrl)
	if err != nil || u.Hostname() == "" {
		panic(fmt.Sprintf("invalid baseUrl: %q", corpGitHubUrl))
	}
	host := fmt.Sprintf("%s://%s", u.Scheme, u.Hostname())
	var corpClient *github.Client
	if host != "" {
		corpClient, err = github.NewClient(nil).WithEnterpriseURLs(
			host,
			strings.ReplaceAll(host, "https://", "https://uploads."),
		)
		if err != nil {
			panic(fmt.Sprintf("can't create GitHub Processor: %s", err))
		}
		corpClient = corpClient.WithAuthToken(corpPat)
	}

	client := github.NewClient(nil)
	if pat != "" {
		client = client.WithAuthToken(pat)
	}

	// Keep  path structure (org/repo/(blob|tree|raw)/branch/optional path ... optional #fragment)
	// Allow optional query/fragment tails in the last groups.
	pattern := "https:\\/\\/((?:[A-Za-z0-9-]+\\.)*(?:github\\.com|github\\.mpi(?:-|\\.)internal\\.com))\\/([^/\\s\"']+)\\/([^/\\s\"']+)\\/(blob|tree|raw)\\/([^/\\s\"']+)(?:\\/([^\\#\\s\\)\\]]*))?(?:\\#([^\\s\\)\\]]+))?"

	urlRegex := regexp.MustCompile(pattern)

	return &InternalLinkProcessor{
		corpClient: corpClient,
		client:     client,
		urlRegex:   urlRegex,
	}
}

func (proc *InternalLinkProcessor) Process(ctx context.Context, url string, logger *zap.Logger) error {
	logger.Debug("Validating internal url", zap.String("url", url))
	match := proc.urlRegex.FindStringSubmatch(url)
	var client *github.Client
	if len(match) == 0 {
		return fmt.Errorf("invalid or unsupported GitHub URL: %s", url)
	}
	host, owner, repo, branch, path, anchor := match[1], match[2], match[3], match[5], strings.TrimPrefix(match[6], "/"), strings.ReplaceAll(match[7], "#", "")
	if host != "github.com" {
		client = proc.corpClient
	} else {
		client = proc.client
	}

	contents, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		var ghError *github.ErrorResponse
		if errors.As(err, &ghError) {
			if ghError.Response.StatusCode == http.StatusNotFound {
				return errs.NewNotFound(url)
			}
		}
		// some other error
		return err
	}
	if contents == nil {
		// contents should not be nil, so something is not ok
		return errs.NewNotFound(url)
	}

	if len(anchor) == 0 {
		// link points to a file or dir, it is found
		return nil
	}

	logger.Debug("Validating anchor in GitHub URL", zap.String("link", url), zap.String("anchor", anchor))
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

func (proc *InternalLinkProcessor) ExtractLinks(line string) []string {
	parts := proc.urlRegex.FindAllString(line, -1)
	if len(parts) == 0 {
		return nil
	}

	urls := make([]string, 0, len(parts))
	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		urls = append(urls, raw)
	}
	return urls
}
