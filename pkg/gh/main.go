// Package 'gh' implements git repository blob|tree|raw|blame|releases|commit links validation
// GitHub links are the links that point to files in other GitHub repositories within the same owner
// (either it is public or enterprise GitHub)
// Example: [README](https://github.com/your-ko/link-validator/blob/main/README.md)
// links to a particular branch or commits are supported as well.

package gh

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
	"time"
)

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

	repoRegex := regexp.MustCompile(
		`^https:\/\/` +
			`(github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*))\/` + // 1 host (no subdomains)
			`([^\/\s"'()<>\[\]{},?#]+)\/` + // 2 org
			`([^\/\s"'()<>\[\]{},?#]+)` + // 3 repo
			`(?:\/` +
			// 4 kind: content OR API-like sections (single capturing group)
			`(blob|tree|raw|blame|releases|commit|issues|pulls|pull|commits|compare|discussions|branches|tags|milestones|labels|projects|actions)` +
			// Optional "tag/" (primarily for releases; harmless if present elsewhere)
			`(?:\/(?:tag\/)?)?` +
			// 5 first segment after kind (ref for content; first tail segment for API)
			`(?:\/([^\/\s"'()<>\[\]{},?#]+))?` +
			// 6 remaining tail (may include '/'; stops before ? or #)
			`(?:\/([^\s"'()<>\[\]{},?#]+))?` +
			`)?` +
			// Optional query is allowed but ignored
			`(?:\?[^\s#"'()<>\[\]{},]*)?` +
			// 7 fragment (optional, without '#')
			`(?:\#([^\s"'()<>\[\]{},?#]+))?` +
			`$`,
	)
	ghRegex := regexp.MustCompile(`(?i)https://github\.(?:com|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)(?:/[^\s"'()<>\[\]{}?#]+)*(?:#[^\s"'()<>\[\]{}]+)?`)

	return &LinkProcessor{
		corpClient: corpClient,
		client:     client,
		repoRegex:  repoRegex,
		ghRegex:    ghRegex,
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

	var fileContent *github.RepositoryContent
	var err error
	switch typ {
	case "blob", "tree", "raw", "blame":
		fileContent, _, _, err = client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
			Ref: ref,
		})
	case "commit":
		_, _, err = client.Repositories.GetCommit(ctx, owner, repo, ref, nil)
	case "releases":
		_, _, err = client.Repositories.GetReleaseByTag(ctx, owner, repo, ref)
	}

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
	if typ == "commit" || typ == "releases" {
		return nil
	}

	if fileContent == nil && typ != "tree" {
		// contents should not be nil, so something is not ok
		return fmt.Errorf("content is nil while it is expected. url: %s. If you think it is a bug, please report it here https://github.com/your-ko/link-validator/issues", url)
	}
	return nil
	//logger.Debug("Validating anchor in GitHub URL", zap.String("link", url), zap.String("anchor", anchor))
	//content, err := fileContent.GetContent()
	//if err != nil {
	//	return err
	//}
	//if !strings.Contains(content, anchor) {
	//	logger.Info("url exists but doesn't have an anchor", zap.String("link", url), zap.String("anchor", anchor))
	//	return errs.NewNotFound(url)
	//} else {
	//	// url with the anchor are correct
	//	return nil
	//}
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
