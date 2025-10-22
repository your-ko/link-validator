package github

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/v74/github"
	"link-validator/pkg/errs"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

type ghHandler func(
	ctx context.Context,
	c *github.Client,
	owner, repo, ref, path string,
) error

func (h ghHandler) String() string {
	if h == nil {
		return "<nil>"
	}
	pc := reflect.ValueOf(h).Pointer()
	if fn := runtime.FuncForPC(pc); fn != nil {
		return fn.Name() // e.g. "github.com/your-org/yourrepo/gh.handleContents"
	}
	return fmt.Sprintf("func@%#x", pc)
}

func handleRepoExist(ctx context.Context, c *github.Client, owner, repo, _, _ string) error {
	_, _, err := c.Repositories.Get(ctx, owner, repo)
	return err
}

func handleContents(ctx context.Context, c *github.Client, owner, repo, ref, path string) error {
	_, _, _, err := c.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
	return err
}

func handleCommit(ctx context.Context, c *github.Client, owner, repo, ref, _ string) error {
	_, _, err := c.Repositories.GetCommit(ctx, owner, repo, ref, &github.ListOptions{})
	return err
}

// handleActionsWorkflows validates the two UI forms:
//   - /actions/workflows/<file>
//   - /actions/workflows/<file>/badge.svg
//
// and I assume that if the workflow exists, then the badge exists too
func handleWorkflow(ctx context.Context, c *github.Client, owner, repo, _, path string) error {
	path = strings.Trim(path, "/")

	if strings.HasSuffix(path, "/badge.svg") {
		path = strings.TrimSuffix(path, "/badge.svg")
	}
	_, _, err := c.Actions.GetWorkflowByFileName(ctx, owner, repo, path)
	return err
}

// handleReleases handles
// /<owner>/<repo>/releases
// /<owner>/<repo>/releases/tag/<tag>
// /<owner>/<repo>/releases/latest
// etc
func handleReleases(ctx context.Context, c *github.Client, owner, repo, ref, path string) error {
	// Normalize for easier branching
	ref = strings.Trim(ref, "/")
	path = strings.Trim(path, "/")
	switch {
	// /<owner>/<repo>/releases  (list page)
	case ref == "" && path == "":
		// again, we assume that if the repo exists, then at least empty list of releases exists as well
		_, _, err := c.Repositories.Get(ctx, owner, repo)
		return err
	// /<owner>/<repo>/releases/tag/<tag>
	case ref == "tag" && path != "":
		_, _, err := c.Repositories.GetReleaseByTag(ctx, owner, repo, path)
		return err
	// /<owner>/<repo>/releases/latest
	case ref == "latest" && path == "":
		_, _, err := c.Repositories.GetLatestRelease(ctx, owner, repo)
		return err
	// Optional: /<owner>/<repo>/releases/download/<tag>/<assetName>
	case ref == "download" && path != "":
		// Validate the tag part exists;
		segs := strings.SplitN(path, "/", 2)
		tag := segs[0]
		_, _, err := c.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
		return err
	// Fallback: some instances use /releases/<tag> without "tag/" — handle that too.
	case ref != "" && path == "":
		_, _, err := c.Repositories.GetReleaseByTag(ctx, owner, repo, ref)
		return err
	}
	return fmt.Errorf("unsupported releases URL variant: ref=%q path=%q", ref, path)
}

func handleIssue(ctx context.Context, c *github.Client, owner, repo, ref, _ string) error {
	n, err := strconv.Atoi(ref)
	if err != nil {
		return fmt.Errorf("invalid issue number %q: %w", ref, err)
	}
	_, _, err = c.Issues.Get(ctx, owner, repo, n)
	return err
}

func handlePR(ctx context.Context, c *github.Client, owner, repo, ref, _ string) error {
	n, err := strconv.Atoi(ref)
	if err != nil {
		return fmt.Errorf("invalid PR number %q: %w", ref, err)
	}
	_, _, err = c.PullRequests.Get(ctx, owner, repo, n)
	return err
}

func mapGHError(url string, err error) error {
	if err == nil {
		return nil
	}
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound {
		return errs.NewNotFound(url)
	}
	return err
}
