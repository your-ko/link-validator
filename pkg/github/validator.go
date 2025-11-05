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
var handlers = map[string]handlerEntry{
	// File-ish routes — all use Contents API
	"blob":  {name: "contents", fn: handleContents},
	"tree":  {name: "contents", fn: handleContents},
	"raw":   {name: "contents", fn: handleContents},
	"blame": {name: "contents", fn: handleContents},

	// Single-object routes
	"commit":   {name: "commit", fn: handleCommit},
	"issues":   {name: "issues", fn: handleIssue},
	"pull":     {name: "pull", fn: handlePR},
	"releases": {name: "releases", fn: handleReleases},
	"actions":  {name: "actions", fn: handleWorkflow},

	// Generic repository routes — we just validate the repo exists
	"pulls":       {name: "repo-exist", fn: handleRepoExist},
	"commits":     {name: "repo-exist", fn: handleRepoExist},
	"discussions": {name: "repo-exist", fn: handleRepoExist},
	"branches":    {name: "repo-exist", fn: handleRepoExist},
	"tags":        {name: "repo-exist", fn: handleRepoExist},
	"milestones":  {name: "repo-exist", fn: handleRepoExist},
	"labels":      {name: "repo-exist", fn: handleRepoExist},
	"projects":    {name: "repo-exist", fn: handleRepoExist},
	"settings":    {name: "repo-exist", fn: handleRepoExist},
}

var (
	ghRegex   = regexp.MustCompile(`(?i)https://github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)(?:/[^\s"'()<>\[\]{}?#]+)*(?:#[^\s"'()<>\[\]{}]+)?`)
	repoRegex = regexp.MustCompile(
		`^https:\/\/` +
			// 1: host (no subdomains like api./uploads.)
			`(github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*))\/` +
			// 2: org
			`([^\/\s"'()<>\[\]{},?#]+)` +
			// optional repo block and everything after it
			`(?:\/` +
			// 3: repo
			`([^\/\s"'()<>\[\]{},?#]+)` +
			// optional kind/ref[/tail...]
			`(?:\/` +
			// 4: kind
			`(blob|tree|raw|blame|releases|commit|issues|pulls|pull|commits|compare|discussions|branches|tags|milestones|labels|projects|actions|settings)` +
			// optional ref section - some URLs like /releases, /pulls, /issues don't require a ref
			`(?:\/` +
			// allow "releases/tag/<ref>" (harmless for others)
			`(?:tag\/)?` +
			// 5: ref or first-after-kind
			`([^\/\s"'()<>\[\]{},?#]+)` +
			// 6: tail (may include multiple / segments)
			`(?:\/([^\s"'()<>\[\]{},?#]+(?:\/[^\s"'()<>\[\]{},?#]+)*))?` +
			`)?` +
			`)?` +
			`)?` +
			// optional trailing slash (for org-only or repo root)
			`\/?` +
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

func New(corpGitHubUrl, corpPat, publicPat string, timeout time.Duration, logger *zap.Logger) (*LinkProcessor, error) {
	client := github.NewClient(httpClient(timeout))
	if publicPat != "" {
		client = client.WithAuthToken(publicPat)
	}
	if corpGitHubUrl == "" {
		return &LinkProcessor{
			client: client,
			logger: logger,
		}, nil
	}

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

	if isCorpUrl(url) && proc.corpGitHubUrl == "" {
		return fmt.Errorf("the url '%s' looks like a corp url, but CORP_URL is not set", url)
	}
	var host, owner, repo, typ, ref, path string

	match := repoRegex.FindStringSubmatch(url)
	if len(match) == 0 {
		return fmt.Errorf("invalid or unsupported GitHub URL: %s. If you think it is a bug, please report it", url)
	}
	host, owner, repo, typ, ref, path, _ = match[1], match[2], strings.TrimSuffix(match[3], ".git"), match[4], match[5], strings.TrimPrefix(match[6], "/"), strings.ReplaceAll(match[7], "#", "")

	client := proc.client
	if host == proc.corpGitHubUrl {
		client = proc.corpClient
	}

	var entry handlerEntry
	var ok bool
	switch {
	case typ == "" || owner == "organizations":
		switch {
		case (owner != "" && repo == "") || owner == "organizations":
			entry = handlerEntry{name: "org-exist", fn: handleOrgExist}
		case owner != "" && repo != "":
			entry = handlerEntry{name: "repo-exist", fn: handleRepoExist}
		default:
			return fmt.Errorf("unsupported GitHub URL: %s", url)
		}
	default:
		entry, ok = handlers[typ]
		if !ok {
			return fmt.Errorf("unsupported GitHub request type %q. Please open an issue", typ)
		}
	}
	proc.logger.Debug("using", zap.String("handler", entry.name))

	return mapGHError(url, entry.fn(ctx, client, owner, repo, ref, path))
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

func isCorpUrl(url string) bool {
	return !strings.Contains(url, "github.com")
}
