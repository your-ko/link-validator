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

	"blob":    {name: "contents", fn: handleContents},
	"tree":    {name: "contents", fn: handleContents},
	"raw":     {name: "contents", fn: handleContents},
	"blame":   {name: "contents", fn: handleContents},
	"compare": {name: "compareCommits", fn: handleCompareCommits},

	// Single-object routes
	"commit":       {name: "commit", fn: handleCommit},
	"pull":         {name: "pull", fn: handlePull},
	"milestone":    {name: "milestone", fn: handleMilestone},
	"project":      {name: "repo-exist", fn: handleRepoExist},
	"advisories":   {name: "advisories", fn: handleSecurityAdvisories},
	"commits":      {name: "repo-exist", fn: handleCommit},
	"actions":      {name: "actions", fn: handleWorkflow},
	"user":         {name: "user", fn: handleUser},
	"attestations": {name: "attestations", fn: handleAttestation},

	// not processed
	"issues":   {name: "issues", fn: handleIssue},
	"releases": {name: "releases", fn: handleReleases},
	"wiki":     {name: "wiki", fn: handleWiki},
	"pkgs":     {name: "pkgs", fn: handlePackages},
	"label":    {name: "labels", fn: handleLabel},

	// Generic lists  — we just validate the repo exists
	"pulls":       {name: "repo-exist", fn: handleRepoExist},
	"labels":      {name: "repo-exist", fn: handleRepoExist},
	"tags":        {name: "repo-exist", fn: handleRepoExist},
	"branches":    {name: "repo-exist", fn: handleRepoExist},
	"settings":    {name: "repo-exist", fn: handleRepoExist},
	"milestones":  {name: "repo-exist", fn: handleRepoExist},
	"discussions": {name: "repo-exist", fn: handleRepoExist},
	"projects":    {name: "repo-exist", fn: handleRepoExist},
	"security":    {name: "repo-exist", fn: handleRepoExist},
	"packages":    {name: "repo-exist", fn: handleRepoExist},
	"orgs":        {name: "org-exist", fn: handleOrgExist},
}

var (
	enterpriseRegex = regexp.MustCompile(`github\.[a-z0-9-]+\.[a-z0-9.-]+`)
	ghRegex         = regexp.MustCompile(`(?i)https://github\.[a-z0-9.-]+(?:/\S*)?`)
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

	gh, err := parseUrl(strings.ToLower(url))
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

	var entry handlerEntry
	var ok bool
	switch gh.typ {
	default:
		entry, ok = handlers[gh.typ]
		if !ok {
			return fmt.Errorf("unsupported GitHub request type %q. Please open an issue", gh.typ)
		}
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
	maxLength := 10
	diff := maxLength - len(parts)
	parts = append(parts, make([]string, diff)...)[:maxLength]

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
	case "branches", "settings", "tags", "labels", "packages":
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
	case "commits", "commit", "issues", "pulls", "pull",
		"discussions", "milestones", "milestone",
		"projects", "project", "advisories", "compare",
		"attestations", "pkgs",
		"actions": // TODO: merge up or move to default
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

		//case "commit", "issues", "pull", , "wiki", "commits":
		//	gh.ref = parts[3]
		//	gh.path = joinPath(parts[4:])
		//case "discussion", "discussions":
		//	// there is no support for discussions in the github sdk, so they need to be fetched by using
		//	// GraphQL API, the REST API, which creates a problem with a separate authentication.
		//	// Maybe will be implemented in the future.
		//	gh.typ = "discussions"
		//case "labels":
		//	if parts[3] != "" {
		//		gh.typ = "label"
		//		gh.ref = parts[3]
		//	}
		//case "releases":
		////https://github.com/your-ko/link-validator/releases/tag/0.9.0
		////https://github.com/your-ko/link-validator/releases/0.9.0
		////https://github.com/your-ko/link-validator/releases
		//case "packages":
		//	// Package URLs: /packages/{package_type}/{package_name}
		//	if parts[3] != "" {
		//		gh.ref = parts[3] // Package type (e.g., "container", "npm", "maven")
		//		if parts[4] != "" {
		//			gh.path = parts[4] // Package name
		//		}
		//	}
		//case :
		//	gh.ref = parts[3]
		//	gh.path = joinPath(parts[4:])
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
