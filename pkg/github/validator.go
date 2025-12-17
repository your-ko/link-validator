// Package 'github' implements git links validation, all links that can be requested vie GitHub API.
// GitHub links are the links that point to files in other GitHub repositories within the same owner
// (either it is public or enterprise GitHub)
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package github

import (
	"context"
	"fmt"
	"link-validator/pkg/regex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v79/github"
)

// handlers is a map from "typ" (blob/tree/raw/…/pulls) to the function.
var handlers = map[string]handlerEntry{
	"nope": {name: "nope", fn: handleNothing},
	"":     {name: "nope", fn: handleNothing},

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
	"repo":         {name: "repo-exist", fn: handleRepoExist},
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
	"search":       {name: "repo-exist", fn: handleRepoExist},
	"orgs":         {name: "org-exist", fn: handleOrgExist},
}

type LinkProcessor struct {
	corpGitHubUrl string
	corpClient    *wrapper
	client        *wrapper
}

func New(corpGitHubUrl, corpPat, publicPat string, timeout time.Duration) (*LinkProcessor, error) {
	client := github.NewClient(httpClient(timeout))
	if publicPat != "" {
		client = client.WithAuthToken(publicPat)
	}
	if corpGitHubUrl == "" {
		return &LinkProcessor{
			client: &wrapper{client},
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
		corpClient:    &wrapper{corpClient},
		client:        &wrapper{client},
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
	slog.Debug("Validating github url", slog.String("url", url))

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
		return fmt.Errorf("unsupported GitHub request type %q. Report an issue", gh.typ)
	}
	slog.Debug("using", slog.String("handler", fmt.Sprintf("github/%s", entry.name)))

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
		// TODO: Improve
		return nil, fmt.Errorf("API/uploads subdomain not supported")
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")

	gh := &ghURL{
		host:       u.Host,
		enterprise: regex.EnterpriseGitHub.MatchString(u.Hostname()),
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
	case "settings", "search", "api":
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
			if gh.owner != "" {
				gh.typ = "user"
			}
		} else {
			gh.typ = "repo"
		}
	case "branches", "settings", "tags", "labels", "packages",
		"pulls", "milestones", "projects", "pkgs", "search":
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
	parts := regex.GitHub.FindAllString(line, -1)
	if len(parts) == 0 {
		return nil
	}

	urls := make([]string, 0, len(parts))
	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue // skip malformed
		}
		if strings.ContainsAny(raw, "[]{}()") {
			continue // seems it is the templated url
		}
		if regex.GitHubExcluded.MatchString(raw) {
			continue // skip non-API GitHub urls
		}

		// Filter out GitHub non-API URLs that shouldn't be validated here
		hostname := strings.ToLower(u.Hostname())
		if hostname == "github.blog" || // GitHub blog
			strings.HasPrefix(hostname, "api.github") || // API endpoints
			strings.HasPrefix(hostname, "uploads.github") || // Upload endpoints
			strings.HasSuffix(hostname, ".githubusercontent.com") || // Raw content CDN
			strings.Contains(raw, "/assets/") {
			continue
		}

		urls = append(urls, raw)
	}

	return urls
}
