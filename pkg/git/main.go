package git

import (
	"context"
	"fmt"
	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
	"regexp"
	"strings"
)

type InternalLinkProcessor struct {
	baseUrl  string
	client   *github.Client
	urlRegex *regexp.Regexp
}

func New(baseUrl, pat string) *InternalLinkProcessor {
	client, err := github.NewClient(nil).WithEnterpriseURLs(baseUrl, strings.ReplaceAll(baseUrl, "https://", "https://uploads."))
	if err != nil {
		panic(fmt.Sprintf("can't create GitHub Processor: %s", err))
	}
	client = client.WithAuthToken(pat)
	sanitised := strings.ReplaceAll(strings.ReplaceAll(baseUrl, ".", "\\."), "/", "\\/")
	urlRegex := regexp.MustCompile(fmt.Sprintf("%s([^\\/]+)\\/([^\\/]+)\\/(blob|tree|raw)\\/([^\\/]+)(?:\\/([^\\#\\s\\)\\]]*))?(\\#[^\\s\\)\\]]+)?", sanitised))

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
	owner, repo, typ, branch, path, anchor := match[1], match[2], match[3], match[4], strings.TrimPrefix(match[5], "/"), match[6]
	fmt.Println(owner, repo, typ, branch, path, anchor) // TODO: remove

	contents, _, _, err := proc.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		// file is not found or some other error
		return err
	}
	if len(anchor) != 0 {
		content, err := contents.GetContent()
		if err != nil {
			return err
		}
		if !strings.Contains(content, anchor) {
			return fmt.Errorf("url '%s' exists but doesn't have an anchor %s", url, anchor)
		} else {
			// file and anchor are found
			return nil
		}
	}
	// file is found
	return nil
}

func (proc *InternalLinkProcessor) Regex() *regexp.Regexp {
	return proc.urlRegex
}
