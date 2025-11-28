// Package 'github' implements git links validation, all links that can be requested vie GitHub API.
// GitHub links are the links that point to files in other GitHub repositories within the same owner
// (either it is public or enterprise GitHub)
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
)

// handlers is a map from "typ" (blob/tree/raw/…/pulls) to the function.
var handlers = map[string]handlerEntry{
	"nope": {name: "nope", fn: handleNothing},
	"":     {name: "repo-exist", fn: handleRepoExist},

	"blob":    {name: "contents", fn: handleContents},
	"tree":    {name: "contents", fn: handleContents},
	"raw":     {name: "contents", fn: handleContents},
	"blame":   {name: "contents", fn: handleContents},
	"compare": {name: "compareCommits", fn: handleCompareCommits},

	// Single-object routes
	"commit":     {name: "commit", fn: handleCommit},
	"pull":       {name: "pull", fn: handlePull},
	"milestone":  {name: "milestone", fn: handleMilestone},
	"advisories": {name: "advisories", fn: handleSecurityAdvisories},
	"commits":    {name: "commit", fn: handleCommit},
	"actions":    {name: "actions", fn: handleWorkflow},
	"user":       {name: "user", fn: handleUser},
	"issues":     {name: "issues", fn: handleIssue},
	"releases":   {name: "releases", fn: handleReleases},
	"label":      {name: "labels", fn: handleLabel},

	// Generic lists  — we just validate the repo exists
	"pulls":        {name: "repo-exist", fn: handleRepoExist},
	"labels":       {name: "repo-exist", fn: handleRepoExist},
	"tags":         {name: "repo-exist", fn: handleRepoExist},
	"branches":     {name: "repo-exist", fn: handleRepoExist},
	"settings":     {name: "repo-exist", fn: handleRepoExist},
	"milestones":   {name: "repo-exist", fn: handleRepoExist},
	"discussions":  {name: "repo-exist", fn: handleRepoExist}, // not available via GitHub API
	"attestations": {name: "repo-exist", fn: handleRepoExist}, // not available via GitHub API
	"wiki":         {name: "wiki", fn: handleWiki},            // not available via GitHub API
	"pkgs":         {name: "pkgs", fn: handlePackages},        // requires authentication, not sure whether it makes sense to implement
	"projects":     {name: "repo-exist", fn: handleRepoExist}, // not available via GitHub API
	"security":     {name: "repo-exist", fn: handleRepoExist},
	"packages":     {name: "repo-exist", fn: handleRepoExist},
	"orgs":         {name: "org-exist", fn: handleOrgExist},
}

var (
	enterpriseRegex = regexp.MustCompile(`github\.[a-z0-9-]+\.[a-z0-9.-]+`)
	gitHubRegex     = regexp.MustCompile(`(?i)https://github\.(?:com|[a-z0-9-]+\.[a-z0-9.-]+)(?:/[^\s\x60\]]*[^\s.,:;!?()\[\]{}\x60])?`)
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

type ghURL struct {
	enterprise bool
	host       string
	owner      string
	repo       string
	typ        string
	ref        string
	path       string
	anchor     string
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, _ string) error {
	proc.logger.Debug("Validating github url", zap.String("url", url))

	gh, err := parseUrl(url)
	if err != nil {
		return err
	}

	if gh.enterprise && proc.corpGitHubUrl == "" {
		return fmt.Errorf("the url '%s' looks like a corp url, but CORP_URL is not set", url)
	}
	client := proc.client
	if gh.host == proc.corpGitHubUrl {
		client = proc.corpClient
	}

	entry, ok := handlers[gh.typ]
	if !ok {
		return fmt.Errorf("unsupported GitHub request type %q. Please open an issue", gh.typ)
	}
	proc.logger.Debug("using", zap.String("handler", entry.name))

	return mapGHError(url, entry.fn(ctx, client, gh.owner, gh.repo, gh.ref, gh.path, gh.anchor))
}

func parseUrl(link string) (*ghURL, error) {
	u, err := url.Parse(strings.TrimSuffix(link, ".git"))
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(u.Hostname(), "github.") {
		return nil, fmt.Errorf("not a GitHub URL")
	}
	if strings.HasSuffix(u.Hostname(), "api.github") ||
		strings.HasPrefix(u.Hostname(), "uploads.github") {
		return nil, fmt.Errorf("API/uploads subdomain not supported")
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")

	gh := &ghURL{
		host:       u.Host,
		enterprise: enterpriseRegex.MatchString(u.Hostname()),
		anchor:     u.Fragment,
	}

	// Handle root GitHub URL
	if len(parts) <= 1 && parts[0] == "" {
		return gh, nil
	}

	// out of size prevention
	growTo := 10 // some arbitrary number more than 5 to prevent 'len out of range' during parsing below
	if len(parts) < growTo {
		diff := growTo - len(parts)
		parts = append(parts, make([]string, diff)...)[:growTo]
	}

	// Handle org urls
	switch parts[0] {
	case "organizations", "orgs":
		gh.typ = "orgs"
		gh.owner = parts[1]
		gh.path = joinPath(parts[2:])
		return gh, nil
	case "settings":
		gh.typ = "nope"
		gh.path = joinPath(parts[1:])
		return gh, nil
	}

	gh.owner = parts[0]
	gh.repo = parts[1]
	gh.typ = parts[2]

	switch gh.typ {
	case "":
		if gh.repo == "" {
			gh.typ = "user"
		}
	case "branches", "settings", "tags", "labels", "packages",
		"pulls", "milestones", "projects", "pkgs":
	// these above go to simple 'if repo exists' validation
	case "blob", "tree", "blame", "raw":
		gh.ref = parts[3]
		gh.path = joinPath(parts[4:])
	case "releases":
		switch parts[3] {
		case "tag", "download":
			gh.ref = parts[3]
			gh.path = joinPath(parts[4:])
		default:
			gh.ref = ""
			gh.path = parts[3]
		}
	case "discussions", "wiki":
		// those might be false positive as they are not available via GitHub API
		gh.ref = parts[3]
		gh.path = joinPath(parts[4:])
	case "commit", "commits", "issues", "pull",
		"milestone", "advisories", "compare",
		"attestations", "actions":
		gh.ref = parts[3]
		gh.path = joinPath(parts[4:])
	case "security":
		// I validate only 'advisories' existence, the rest is goes by 'handleRepoExist'
		if parts[3] == "advisories" && parts[4] != "" {
			gh.typ = "advisories"
			gh.ref = parts[4]
		}
	default:
		return nil, fmt.Errorf("unsupported GitHub URL found '%s', please report a bug", link)
	}

	return gh, nil
}

func joinPath(parts []string) string {
	i := 0
	for ; i < len(parts) && parts[i] != ""; i++ {
	} // find first empty
	if i == 0 {
		return ""
	}
	if i == 1 {
		return parts[0]
	}
	return strings.Join(parts[:i], "/")
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := gitHubRegex.FindAllString(line, -1)
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
