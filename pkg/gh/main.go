// Package 'gh' implements git repository blob|tree|raw|blame|releases|commit links validation
// GitHub links are the links that point to files in other GitHub repositories within the same owner
// (either it is public or enterprise GitHub)
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package gh

import (
	"context"
	"fmt"
	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ghHandler func(
	ctx context.Context,
	c *github.Client,
	owner, repo, ref, path string,
) error

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
}

type LinkProcessor struct {
	corpGitHubUrl string
	corpClient    *github.Client
	client        *github.Client
	repoRegex     *regexp.Regexp
	ghRegex       *regexp.Regexp
}

func New(corpGitHubUrl, corpPat, pat string) *LinkProcessor {
	// Derive the bare host from baseUrl, e.g. "github.mycorp.com"
	u, err := url.Parse(corpGitHubUrl)
	if err != nil || u.Hostname() == "" {
		panic(fmt.Sprintf("invalid enterprise url: %q", corpGitHubUrl))
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

	repoRegex := regexp.MustCompile(
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

	ghRegex := regexp.MustCompile(`(?i)https://github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)(?:/[^\s"'()<>\[\]{}?#]+)*(?:#[^\s"'()<>\[\]{}]+)?`)

	return &LinkProcessor{
		corpGitHubUrl: u.Hostname(),
		corpClient:    corpClient,
		client:        client,
		repoRegex:     repoRegex,
		ghRegex:       ghRegex,
	}
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, logger *zap.Logger) error {
	logger.Debug("Validating internal url", zap.String("url", url))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	match := proc.repoRegex.FindStringSubmatch(url)
	var client *github.Client
	if len(match) == 0 {
		return fmt.Errorf("invalid or unsupported GitHub URL: %s. If you think it is a bug, please report it here https://github.com/your-ko/link-validator/issues", url)
	}

	host, owner, repo, typ, ref, path, _ := match[1], match[2], match[3], match[4], match[5], strings.TrimPrefix(match[6], "/"), strings.ReplaceAll(match[7], "#", "")
	if host == proc.corpGitHubUrl {
		client = proc.corpClient
	} else {
		client = proc.client
	}

	handler, ok := handlers[typ]
	if !ok {
		return fmt.Errorf("unsupported GitHub request type %q; please open an issue", typ)
	}

	return mapGHError(url, handler(ctx, client, owner, repo, ref, path))
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := proc.ghRegex.FindAllString(line, -1)
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
