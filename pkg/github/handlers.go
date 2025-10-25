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

type handlerEntry struct {
	name string
	fn   ghHandler
}

// handleRepoExist validates the repository existence.
//
// GitHub API docs: https://docs.github.com/rest/repos/repos#get-a-repository
//
//meta:operation GET /repos/{owner}/{repo}
func handleRepoExist(ctx context.Context, c *github.Client, owner, repo, _, _ string) error {
	_, _, err := c.Repositories.Get(ctx, owner, repo)
	return err
}

// handleOrgExist  validates the org existence.
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs#get-an-organization
//
//meta:operation GET /orgs/{org}
func handleOrgExist(ctx context.Context, c *github.Client, owner, _, _, _ string) error {
	_, _, err := c.Organizations.Get(ctx, owner)
	return err
}

// handleContents validates existence either the metadata and content of a single file or subdirectories of a directory
//
//meta:operation GET /repos/{owner}/{repo}/contents/{path}
func handleContents(ctx context.Context, c *github.Client, owner, repo, ref, path string) error {
	_, _, _, err := c.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
	return err
}

// handleCommit validates existence of the specified commit.
//
// GitHub API docs: https://docs.github.com/rest/commits/commits#get-a-commit
//
//meta:operation GET /repos/{owner}/{repo}/commits/{ref}
func handleCommit(ctx context.Context, c *github.Client, owner, repo, ref, _ string) error {
	_, _, err := c.Repositories.GetCommit(ctx, owner, repo, ref, &github.ListOptions{})
	return err
}

// handleWorkflow validates the two UI forms:
//   - /actions/workflows/<file>
//   - /actions/workflows/<file>/badge.svg
//
// and I assume that if the workflow exists, then the badge exists too
func handleWorkflow(ctx context.Context, c *github.Client, owner, repo, _, path string) error {
	path = strings.TrimSuffix(path, "/badge.svg")
	path = strings.Trim(path, "/")

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

// handleIssue validates existence of a single issue.
//
// GitHub API docs: https://docs.github.com/rest/issues/issues#get-an-issue
//
//meta:operation GET /repos/{owner}/{repo}/issues/{issue_number}
func handleIssue(ctx context.Context, c *github.Client, owner, repo, ref, _ string) error {
	n, err := strconv.Atoi(ref)
	if err != nil {
		return fmt.Errorf("invalid issue number %q: %w", ref, err)
	}
	_, _, err = c.Issues.Get(ctx, owner, repo, n)
	return err
}

// handlePR validates existence of a single pull request.
//
// GitHub API docs: https://docs.github.com/rest/pulls/pulls#get-a-pull-request
//
//meta:operation GET /repos/{owner}/{repo}/pulls/{pull_number}
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
