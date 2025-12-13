package github

import (
	"context"
	"errors"
	"fmt"
	"link-validator/pkg/errs"
	"link-validator/pkg/regex"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/go-github/v77/github"
)

type ghHandler func(
	ctx context.Context,
	c client,
	owner, repo, ref, path, fragment string,
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

// handleNope does nothing (quite exciting, right?).
// It always returns true. Useful for generic GitHub urls
func handleNothing(_ context.Context, _ client, _, _, _, _, _ string) error {
	return nil
}

// handleRepoExist validates the repository existence.
//
// GitHub API docs: https://docs.github.com/rest/repos/repos#get-a-repository
//
//meta:operation GET /repos/{owner}/{repo}
func handleRepoExist(ctx context.Context, c client, owner, repo, _, _, _ string) error {
	_, _, err := c.getRepository(ctx, owner, repo)
	return err
}

// handleContents validates existence either the metadata and content of a single file or subdirectories of a directory
//
//meta:operation GET /repos/{owner}/{repo}/contents/{path}
func handleContents(ctx context.Context, c client, owner, repo, ref, path, _ string) error {
	if strings.HasPrefix(path, "heads/") {
		// extract the branch name
		parts := strings.SplitN(strings.TrimPrefix(path, "heads/"), "/", 2)
		ref = parts[0]
		path = parts[1]
	}
	_, _, _, err := c.getContents(ctx, owner, repo, ref, path)
	return err
}

// handleCommit validates existence of the specified commit.
//
// GitHub API docs: https://docs.github.com/rest/commits/commits#get-a-commit
//
//meta:operation GET /repos/{owner}/{repo}/commits/{ref}
func handleCommit(ctx context.Context, c client, owner, repo, ref, _, _ string) error {
	_, _, err := c.getCommit(ctx, owner, repo, ref, nil)
	return err
}

// handleCompareCommits validates existence of the specified commit.
//
// GitHub API docs: https://docs.github.com/rest/commits/commits#get-a-commit
//
//meta:operation GET /repos/{owner}/{repo}/compare/{basehead}
func handleCompareCommits(ctx context.Context, c client, owner, repo, ref, path, fragment string) error {
	if ref == "" {
		//	https://github.com/your-ko/link-validator/compare is a valid link
		return handleRepoExist(ctx, c, owner, repo, ref, path, fragment)
	}
	parts := regex.DotPattern.Split(ref, -1)
	var left, right string
	switch len(parts) {
	case 2:
		left = parts[0]
		right = parts[1]
	case 1:
		right = parts[0]
		repository, _, err := c.getRepository(ctx, owner, repo)
		if err != nil {
			return err
		}
		left = *repository.DefaultBranch
	default:
		// should not happen
		return fmt.Errorf("incorrect GitHub compare URL, expected '/repos/{owner}/{repo}/compare/{basehead}'")
	}
	_, _, err := c.compareCommits(ctx, owner, repo, left, right, nil)
	return err
}

// handlePull validates existence of a single pull request.
//
// GitHub API docs: https://docs.github.com/rest/pulls/pulls#get-a-pull-request
//
//meta:operation GET /repos/{owner}/{repo}/pulls/{pull_number}
func handlePull(ctx context.Context, c client, owner, repo, ref, path, fragment string) error {
	prNumber, err := strconv.Atoi(ref)
	if err != nil {
		return fmt.Errorf("invalid PR number %q", ref)
	}
	_, _, err = c.getPR(ctx, owner, repo, prNumber)
	if err != nil {
		return err
	}
	if fragment == "" && !strings.ContainsRune(path, '/') {
		// presumably, if PR exists, then the list of files/commits/checks exist as well
		return nil
	}

	if strings.HasPrefix(path, "commits") {
		//https://github.com/your-ko/link-validator/pull/280/commits/9130a7d501f28318d2761756f18b993b626181fa
		//https://github.com/your-ko/link-validator/pull/280/commits/9130a7d501f28318d2761756f18b993b626181fa#diff-72ad4ae8af5a8d5be342d002cedff28908ba09b42e17197ed14b62081e62141dR31
		SHA := strings.Split(path, "/")[1]
		commits, _, err := c.listCommits(ctx, owner, repo, prNumber, nil)
		if err != nil {
			return err
		}
		for _, commit := range commits {
			if commit.GetSHA() == SHA {
				// unfortunately there is no API to find diff by SHA, so I ignore that bit
				return nil
			}
		}
		return fmt.Errorf("commit '%s' not found in PR '%s'", SHA, ref)
	}

	// Handle fragments
	if strings.HasPrefix(fragment, "issuecomment-") {
		// Handle issue comments: #issuecomment-<id>
		commentId, err := strconv.ParseInt(strings.TrimPrefix(fragment, "issuecomment-"), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid comment id: '%s'", fragment)
		}
		_, _, err = c.getIssueComment(ctx, owner, repo, commentId)
		return err
	} else if strings.HasPrefix(fragment, "discussion_r") {
		// Handle review comments: #discussion_r<id>
		commentId, err := strconv.ParseInt(strings.TrimPrefix(fragment, "discussion_r"), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid comment id: '%s'", fragment)
		}
		_, _, err = c.getPRComment(ctx, owner, repo, commentId)
		return err
	} else if strings.HasPrefix(fragment, "diff-") {
		// https://github.com/your-ko/link-validator/pull/280/commits/9130a7d501f28318d2761756f18b993b626181fa#diff-72ad4ae8af5a8d5be342d002cedff28908ba09b42e17197ed14b62081e62141dL31
		// https://github.com/your-ko/link-validator/pull/280/files#diff-48192d841d01a270fcd26b6e06f1e886333860b7f1ce32ae5758338d3c6551f7R10
		// unfortunately there is no API to find diff by SHA, so I mark the URL as correct
		return nil
	}

	return fmt.Errorf("unsupported PR fragment format: '%s'. Please report a bug", fragment)
}

// handleMilestone validates existence of a single milestone.
//
// GitHub API docs: https://docs.github.com/rest/issues/milestones#get-a-milestone
//
//meta:operation GET /repos/{owner}/{repo}/milestones/{milestone_number}
func handleMilestone(ctx context.Context, c client, owner, repo, ref, _, _ string) error {
	n, err := strconv.Atoi(ref)
	if err != nil {
		return fmt.Errorf("invalid milestone number %q", ref)
	}
	_, _, err = c.getMilestone(ctx, owner, repo, n)
	return err
}

// handleSecurityAdvisories validates existence of security advisories.
// For the URL pattern: /security/advisories/{advisory_id}
//
// GitHub API docs: https://docs.github.com/rest/security-advisories/repository-advisories
//
//meta:operation GET /repos/{owner}/{repo}/security-advisories
func handleSecurityAdvisories(ctx context.Context, c client, owner, repo, ref, _, _ string) error {
	if ref == "" {
		return fmt.Errorf("security advisory ID is required")
	}

	// Since there's no direct GetRepositoryAdvisory method, I list all advisories
	advisories, _, err := c.listRepositorySecurityAdvisories(ctx, owner, repo, nil)
	if err != nil {
		return err
	}

	for _, advisory := range advisories {
		if advisory.GetGHSAID() == ref {
			return nil // Found the advisory
		}
	}

	return errs.NewNotFoundMessage(fmt.Sprintf("security advisory %q not found", ref))
}

// handleWorkflow validates the two UI forms:
//   - /actions/workflows/<file>
//   - /actions/workflows/<file>/badge.svg
//
// and I assume that if the workflow exists, then the badge exists too
func handleWorkflow(ctx context.Context, c client, owner, repo, ref, path, fragment string) error {
	switch {
	case path == "":
		// presumably if the repo exists then the actions list exists as well
		return handleRepoExist(ctx, c, owner, repo, ref, path, fragment)
	case ref == "workflows":
		path = strings.TrimSuffix(path, "/badge.svg")
		_, _, err := c.getWorkflowByFileName(ctx, owner, repo, path)
		return err
	case ref == "runs":
		parts := strings.Split(path, "/")
		runId, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid workflow id: '%s'", path)
		}

		switch {
		case strings.Contains(path, "job"):
			job := strings.TrimPrefix(path, fmt.Sprintf("%v/job/", runId))
			jobId, err := strconv.ParseInt(job, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid job id: '%s'", path)
			}
			_, _, err = c.getWorkflowJobByID(ctx, owner, repo, jobId)
			return err
		case strings.Contains(path, "attempts"):
			attempts := strings.TrimPrefix(path, fmt.Sprintf("%v/attempts/", runId))
			attemptId, err := strconv.ParseInt(attempts, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid attempt id: '%s'", path)
			}
			_, _, err = c.listWorkflowJobsAttempt(ctx, owner, repo, runId, attemptId, &github.ListOptions{})
			return err
		default:
			_, _, err = c.getWorkflowRunByID(ctx, owner, repo, runId)
			return err
		}
	}
	return fmt.Errorf("unsupported ref found, please report a bug")
}

// handleUser validater user existence
//
// GitHub API docs: https://docs.github.com/rest/users/users#get-a-user
// GitHub API docs: https://docs.github.com/rest/users/users#get-the-authenticated-user
//
//meta:operation GET /user
//meta:operation GET /users/{username}
func handleUser(ctx context.Context, c client, owner, _, _, _, _ string) error {
	_, _, err := c.getUsers(ctx, owner)
	return err
}

// handleIssue validates existence of a single issue.
//
// GitHub API docs: https://docs.github.com/rest/issues/issues#get-an-issue
//
//meta:operation GET /repos/{owner}/{repo}/issues/{issue_number}
func handleIssue(ctx context.Context, c client, owner, repo, ref, path, fragment string) error {
	if ref == "" {
		return handleRepoExist(ctx, c, owner, repo, ref, path, fragment)
	}
	n, err := strconv.Atoi(ref)
	if err != nil {
		return fmt.Errorf("invalid issue number %q", ref)
	}
	_, _, err = c.getIssue(ctx, owner, repo, n)
	return err
}

// handleReleases handles
// /<owner>/<repo>/releases
// /<owner>/<repo>/releases/tag/<tag>
// /<owner>/<repo>/releases/latest
// etc
func handleReleases(ctx context.Context, c client, owner, repo, ref, path, fragment string) error {
	switch {
	case path == "latest":
		_, _, err := c.getLatestRelease(ctx, owner, repo)
		return err
	case path == "":
		// presumably if the repo exists then the releases list exists as well
		return handleRepoExist(ctx, c, owner, repo, ref, path, fragment)
	case ref == "tag":
		_, _, err := c.getReleaseByTag(ctx, owner, repo, path)
		return err
	case ref == "download":
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			return fmt.Errorf("incorrect download path '%s' in the release url", path)
		}
		r, _, err := c.getReleaseByTag(ctx, owner, repo, parts[0])
		if err != nil {
			return err
		}
		for _, asset := range r.Assets {
			if *asset.Name == parts[1] {
				// we found an asset in the release
				return nil
			}
		}
		return errs.NewNotFoundMessage(fmt.Sprintf("asset '%s' wasn't found in the release assets", parts[1]))
	}
	return fmt.Errorf("unexpected release path '%s' found. Please report a bug", path)
}

// handleLabel validates existence of a label.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels#list-labels-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/labels
func handleLabel(ctx context.Context, c client, owner, repo, ref, _, _ string) error {
	labels, _, err := c.listLabels(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		return err
	}
	for _, l := range labels {
		if *l.Name == ref {
			return nil
		}
	}
	return errs.NewNotFoundMessage(fmt.Sprintf("label '%s' not found", ref))
}

// handleWiki validates existence of GitHub wiki pages.
// For the URL pattern: /wiki/{page-name}
//
// Note: GitHub wikis are not accessible through the REST API, so we can only
// validate that the repository exists and has wiki enabled.
// Handles different wiki URL patterns:
// - /wiki (wiki home page)
// - /wiki/{page-name} (specific wiki page)
func handleWiki(ctx context.Context, c client, owner, repo, _, _, _ string) error {
	repository, _, err := c.getRepository(ctx, owner, repo)
	if err != nil {
		return err
	}

	// Check if wiki is enabled for this repository
	if !repository.GetHasWiki() {
		return errs.NewNotFoundMessage(fmt.Sprintf("wiki is not enabled for repository %s/%s", owner, repo))
	}

	return nil
}

// handlePackages validates existence of GitHub packages.
// Since GetPackage requires user authentication, it is not suitable for link-validator,
// that's why it always returns true
//
// For the URL pattern: /packages/{package_type}/{package_name}
//
// GitHub API docs: https://docs.github.com/rest/packages/packages
//
//meta:operation GET /user/packages/{package_type}/{package_name}
//meta:operation GET /users/{username}/packages/{package_type}/{package_name}
func handlePackages(ctx context.Context, c client, owner, repo, packageType, packageName, fragment string) error {
	return handleRepoExist(ctx, c, owner, repo, packageType, packageName, fragment)
	// Handle different package URL patterns:
	// - /packages/{package_type}/{package_name} (specific package)

	//if packageType == "" {
	//	return fmt.Errorf("package type is required")
	//}
	//
	//if packageName == "" {
	//	return fmt.Errorf("package name is required")
	//}

	// Try to get the package from the user/organization
	// First, try as a user package
	//_, _, err := c.Users.GetPackage(ctx, owner, packageType, packageName)
	//if err == nil {
	//	return nil // Package found as user package
	//}
	//
	//_, _, orgErr := c.Organizations.GetPackage(ctx, owner, packageType, packageName)
	//if orgErr == nil {
	//	return nil // Package found as organization package
	//}

	//return nil
}

// handleOrgExist  validates the org existence.
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs#get-an-organization
//
//meta:operation GET /orgs/{org}
func handleOrgExist(ctx context.Context, c client, owner, _, _, _, _ string) error {
	if owner == "" {
		return nil
	}
	_, _, err := c.getOrganization(ctx, owner)
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
	if errors.Is(err, errs.ErrNotFound) {
		return err
	}
	return err
}
