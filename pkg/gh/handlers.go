package gh

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/v74/github"
	"link-validator/pkg/errs"
	"net/http"
	"strconv"
	"strings"
)

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
// regex captures (as in your current parser):
//
//	typ = "actions", ref = "workflows", path = "<file>" or "<file>/badge.svg"
func handleWorkflow(ctx context.Context, c *github.Client, owner, repo, ref, path string) error {
	ref = strings.ToLower(strings.Trim(ref, "/"))
	path = strings.Trim(path, "/")

	// We only specialize "actions/workflows/...".
	if ref != "workflows" || path == "" {
		// For other actions pages, just validate repo existence.
		_, _, err := c.Repositories.Get(ctx, owner, repo)
		return err
	}

	// /actions/workflows/<file>/badge.svg
	if strings.HasSuffix(path, "/badge.svg") {
		wfFile := strings.TrimSuffix(path, "/badge.svg")

		wf, _, err := c.Actions.GetWorkflowByFileName(ctx, owner, repo, wfFile)
		if err != nil {
			return err
		}

		// hit the badge endpoint: GET /repos/{owner}/{repo}/actions/workflows/{id}/badge
		// go-github doesn’t wrap this as a typed call, so use NewRequest/Do.
		u := fmt.Sprintf("repos/%s/%s/actions/workflows/%d/badge", owner, repo, wf.GetID())
		req, err := c.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return err
		}

		_, err = c.Do(ctx, req, nil) // we only validate status
		return err
	}

	// /actions/workflows/<file>
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
