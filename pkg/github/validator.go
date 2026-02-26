// Package 'github' implements git links validation, all links that can be requested vie GitHub API.
// GitHub links are the links that point to files in other GitHub repositories within the same owner
// (either it is public or enterprise GitHub)
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package github

import (
	"context"
	"fmt"
	"link-validator/pkg/config"
	httpvalidator "link-validator/pkg/http"
	"link-validator/pkg/regex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v83/github"
)

// handlers is a map from "typ" (blob/tree/raw/…/pulls) to the function.
var handlers = map[string]handlerEntry{
	"nope": {"nope", APIHandler{fn: handleNothing}},
	"":     {"nope", APIHandler{fn: handleNothing}},

	"blob":    {"contents", APIHandler{fn: handleContents}},
	"tree":    {"contents", APIHandler{fn: handleContents}},
	"raw":     {"contents", APIHandler{fn: handleContents}},
	"blame":   {"contents", APIHandler{fn: handleContents}},
	"compare": {"compareCommits", APIHandler{fn: handleCompareCommits}},

	// Single-object routes
	"commit":       {"commit", APIHandler{fn: handleCommit}},
	"pull":         {"pull", APIHandler{fn: handlePull}},
	"milestone":    {"milestone", APIHandler{fn: handleMilestone}},
	"advisories":   {"advisories", APIHandler{fn: handleSecurityAdvisories}},
	"commits":      {"commit", APIHandler{fn: handleCommit}},
	"actions":      {"actions", APIHandler{fn: handleWorkflow}},
	"user":         {"user", APIHandler{fn: handleUser}},
	"issues":       {"issues", APIHandler{fn: handleIssue}},
	"releases":     {"releases", APIHandler{fn: handleReleases}},
	"labels":        {"labels", APIHandler{fn: handleLabel}},
	"gist":         {"gist", APIHandler{fn: handleGist}},
	"environments": {"environments", APIHandler{fn: handleEnvironments}},
	"teams":        {"teams", APIHandler{fn: handleTeams}},

	// Generic lists  — we just validate the repo exists
	"repo":       {"repo-exist", APIHandler{fn: handleRepoExist}},
	"pulls":      {"repo-exist", APIHandler{fn: handleRepoExist}},
	"tags":       {"repo-exist", APIHandler{fn: handleRepoExist}},
	"branches":   {"repo-exist", APIHandler{fn: handleRepoExist}},
	"settings":   {"repo-exist", APIHandler{fn: handleRepoExist}},
	"milestones": {"repo-exist", APIHandler{fn: handleRepoExist}},
	"pkgs":       {"pkgs", APIHandler{fn: handlePackages}},
	"packages":   {"packages", APIHandler{fn: handlePackages}},
	//"projects":     { "repo-exist", APIHandler{fn:  handleRepoExist}, // Classic GitHub projects are not available via GitHub API, only ProjectV2
	"security":     {"repo-exist", APIHandler{fn: handleRepoExist}},
	"search":       {"repo-exist", APIHandler{fn: handleRepoExist}},
	"orgs":         {"org-exist", APIHandler{fn: handleOrgExist}},
	"attestations": {"attestations", HTTPHandler{fn: handleHttp}}, // HTTP-based validation
	"wiki":         {"wiki", HTTPHandler{fn: handleHttp}},         // HTTP-based validation
	"projects":     {"projects", HTTPHandler{fn: handleHttp}},     // HTTP-based validation
	"discussions":  {"discussions", HTTPHandler{fn: handleHttp}},  // not available via GitHub API
	"assets":       {"assets", HTTPHandler{fn: handleHttp}},       // CDN assets, HTTP-only
}

type LinkProcessor struct {
	corpGitHubUrl string
	corpClient    *wrapper
	client        *wrapper
	httpClient    *http.Client
}

func New(cfg *config.Config) (*LinkProcessor, error) {
	httpClient := httpvalidator.InitHttpClient(cfg)

	client := github.NewClient(httpClient)
	if cfg.Validators.GitHub.PAT != "" {
		client = client.WithAuthToken(cfg.Validators.GitHub.PAT)
	}
	if cfg.Validators.GitHub.CorpGitHubUrl == "" {
		return &LinkProcessor{
			client:     &wrapper{client},
			httpClient: httpClient,
		}, nil
	}

	// Derive the bare host from baseUrl, e.g. "github.mycorp.com"
	u, err := url.Parse(cfg.Validators.GitHub.CorpGitHubUrl)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid enterprise url: '%s'", cfg.Validators.GitHub.CorpGitHubUrl)
	}
	host := fmt.Sprintf("%s://%s", u.Scheme, u.Hostname())
	corpClient, err := github.NewClient(httpClient).WithEnterpriseURLs(
		host,
		strings.ReplaceAll(host, "https://", "https://uploads."),
	)
	if err != nil {
		return nil, fmt.Errorf("can't create GitHub Processor: %s", err)
	}
	corpClient = corpClient.WithAuthToken(cfg.Validators.GitHub.CorpPAT)

	return &LinkProcessor{
		corpGitHubUrl: u.Hostname(),
		corpClient:    &wrapper{corpClient},
		client:        &wrapper{client},
		httpClient:    httpClient,
	}, nil
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
	url        string
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, _ string) error {
	slog.Debug("github: starting validation:", slog.String("url", url))

	gh, err := parseUrl(url)
	if err != nil {
		return err
	}

	if gh.enterprise && proc.corpGitHubUrl == "" {
		return fmt.Errorf("the url '%s' looks like a corp url, but CORP_URL is not set", url)
	}
	client := proc.client
	if proc.corpGitHubUrl == strings.TrimPrefix(gh.host, "gist.") {
		client = proc.corpClient
	}
	entry, ok := handlers[gh.typ]
	if !ok {
		return fmt.Errorf("unsupported GitHub request type %q. Report an issue", gh.typ)
	}
	slog.Debug("github: using", slog.String("handler", entry.name))

	return mapGHError(url, entry.handler.Handle(ctx, client, proc.httpClient, gh))
}

func parseUrl(link string) (*ghURL, error) {
	u, err := url.Parse(strings.TrimSuffix(link, ".git"))
	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")

	gh := &ghURL{
		host:       u.Host,
		enterprise: regex.EnterpriseGitHub.MatchString(u.Hostname()),
		anchor:     u.Fragment,
		url:        link,
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
		if parts[2] != "teams" {
			gh.typ = "orgs"
			gh.owner = parts[1]
			gh.path = joinPath(parts[2:])
			return gh, nil
		}
	case "users":
		gh.typ = parts[2] // "projects", etc.
		gh.owner = parts[1]
		if len(parts) > 3 && parts[3] != "" {
			gh.ref = parts[3]
			gh.path = joinPath(parts[4:])
		}
		return gh, nil
	case "settings", "search", "api":
		gh.typ = "nope"
		gh.path = joinPath(parts[1:])
		return gh, nil
	}

	gh.owner = parts[0]
	gh.repo = parts[1]
	gh.typ = parts[2]

	// Handle gist.github.com URLs
	if strings.HasPrefix(u.Hostname(), "gist.") {
		if len(parts) < 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid gist URL: missing user/gist parts")
		}
		gh.typ = "gist"
	}

	switch gh.typ {
	case "":
		if gh.repo == "" {
			if gh.owner != "" {
				gh.typ = "user"
			}
		} else {
			gh.typ = "repo"
		}
	case "branches", "tags", "labels", "pulls", "milestones", "search":
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
	case "packages", "pkgs":
		if gh.typ == "pkgs" {
			gh.ref = parts[3] // package type (container, npm, etc.)
			gh.path = joinPath(parts[4:])
		} else {
			// "packages" - just list packages for repo
			gh.ref = ""
			gh.path = joinPath(parts[3:])
		}
	case "discussions", "wiki", "projects", "assets":
		// those might be false positive as they are not available via GitHub API
		gh.ref = parts[3]
		gh.path = joinPath(parts[4:])
	case "commit", "commits", "issues", "pull",
		"milestone", "advisories", "compare",
		"attestations", "actions", "environments":
		gh.ref = parts[3]
		gh.path = joinPath(parts[4:])
	case "settings":
		if parts[3] == "environments" {
			gh.typ = parts[3]
			gh.ref = parts[4]
			gh.path = parts[5]
		}
	case "teams":
		gh.owner = parts[1]
		gh.repo = ""
		gh.typ = parts[2]
		gh.ref = parts[3]
	case "gist":
		gh.ref = parts[2]
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
		if strings.HasPrefix(raw, "https://github.com/features") {
			continue // marketing pages, no repo context — handled by HTTP processor
		}

		urls = append(urls, strings.TrimPrefix(raw, "/"))
	}

	return urls
}

func (proc *LinkProcessor) Excludes(url string) bool {
	// github.com/features/* are marketing pages with no repo context
	if strings.HasPrefix(url, "https://github.com/features") {
		return false
	}
	// github.blog, docs.github.com, raw.githubusercontent.com don't match
	// regex.GitHub so they're already handled by the HTTP processor
	return regex.GitHub.MatchString(url)
}
