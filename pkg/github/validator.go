// Package 'github' implements git links validation, all links that can be requested vie GitHub API.
// GitHub links are the links that point to files in other GitHub repositories within the same owner
// (either it is public or enterprise GitHub)
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package github

import (
	"context"
	"fmt"
	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// handlers is a map from "typ" (blob/tree/raw/…/pulls) to the function.
var handlers = map[string]ghHandler{
	// File-ish routes — all use Contents API
	"blob":  handleContents,
	"tree":  handleContents,
	"raw":   handleContents,
	"blame": handleContents,

	// Single-object routes
	"commit":   handleCommit,
	"issues":   handleIssue,
	"pull":     handlePR,
	"releases": handleReleases,
	"actions":  handleWorkflow,

	// “List / page” routes — we just validate the repo exists
	"pulls":       handleRepoExist,
	"commits":     handleRepoExist,
	"discussions": handleRepoExist,
	"branches":    handleRepoExist,
	"tags":        handleRepoExist,
	"milestones":  handleRepoExist,
	"labels":      handleRepoExist,
	"projects":    handleRepoExist,
	"":            handleRepoExist,
}

var (
	ghRegex   = regexp.MustCompile(`(?i)https://github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)(?:/[^\s"'()<>\[\]{}?#]+)*(?:#[^\s"'()<>\[\]{}]+)?`)
	repoRegex = regexp.MustCompile(
		`^https:\/\/` +
			// 1: host (no subdomains like api./uploads.)
			`(github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*))\/` +
			// 2: org
			`([^\/\s"'()<>\[\]{},?#]+)\/` +
			// 3: repo
			`([^\/\s"'()<>\[\]{},?#]+)` +
			// allow repo root with or without trailing slash
			`\/?` +
			// optionally: 4 kind + 5 ref/first + 6 tail (now allows multiple segments)
			`(?:\/` +
			`(blob|tree|raw|blame|releases|commit|issues|pulls|pull|commits|compare|discussions|branches|tags|milestones|labels|projects|actions)` + `\/` +
			`(?:tag\/)?` + // lets "releases/tag/<tag>" work
			`([^\/\s"'()<>\[\]{},?#]+)` + // 5: ref or first segment after kind
			`(?:\/([^\s"'()<>\[\]{},?#]+(?:\/[^\s"'()<>\[\]{},?#]+)*))?` + // 6: tail (may include multiple / segments)
			`)?` +
			// 7: optional fragment
			`(?:\#([^\s"'()<>\[\]{},?#]+))?` +
			`$`,
	)
)

type LinkProcessor struct {
	corpGitHubUrl string
	corpClient    *github.Client
	client        *github.Client
	logger        *zap.Logger
}

func New(corpGitHubUrl, corpPat, pat string, timeout time.Duration, logger *zap.Logger) (*LinkProcessor, error) {
	// Derive the bare host from baseUrl, e.g. "github.mycorp.com"
	u, err := url.Parse(corpGitHubUrl)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid enterprise url: '%s'", corpGitHubUrl)
	}
	host := fmt.Sprintf("%s://%s", u.Scheme, u.Hostname())
	var corpClient *github.Client
	if host != "" {
		corpClient, err = github.NewClient(httpClient(timeout)).WithEnterpriseURLs(
			host,
			strings.ReplaceAll(host, "https://", "https://uploads."),
		)
		if err != nil {
			return nil, fmt.Errorf("can't create GitHub Processor: %s", err)
		}
		corpClient = corpClient.WithAuthToken(corpPat)
	}

	client := github.NewClient(httpClient(timeout))
	if pat != "" {
		client = client.WithAuthToken(pat)
	}

	return &LinkProcessor{
		corpGitHubUrl: u.Hostname(),
		corpClient:    corpClient,
		client:        client,
		logger:        logger,
	}, nil
}

func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, _ string) error {
	proc.logger.Debug("Validating github url", zap.String("url", url))

	match := repoRegex.FindStringSubmatch(url)
	var client *github.Client
	if len(match) == 0 {
		return fmt.Errorf("invalid or unsupported GitHub URL: %s. If you think it is a bug, please report it", url)
	}

	host, owner, repo, typ, ref, path, _ := match[1], match[2], match[3], match[4], match[5], strings.TrimPrefix(match[6], "/"), strings.ReplaceAll(match[7], "#", "")
	if host == proc.corpGitHubUrl {
		client = proc.corpClient
	} else {
		client = proc.client
	}

	handler, ok := handlers[typ]
	if !ok {
		return fmt.Errorf("unsupported GitHub request type %q. If you think it is a bug, please report it", typ)
	}
	proc.logger.Debug("using", zap.Any("handler", handler))

	return mapGHError(url, handler(ctx, client, owner, repo, ref, path))
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := ghRegex.FindAllString(line, -1)
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
