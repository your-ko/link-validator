package github

import (
	"context"

	"github.com/google/go-github/v80/github"
)

type client interface {
	getRepository(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
	getContents(ctx context.Context, owner, repo, ref, path string) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error)
	getCommit(ctx context.Context, owner, repo, sha string, opts *github.ListOptions) (*github.RepositoryCommit, *github.Response, error)
	compareCommits(ctx context.Context, owner, repo, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error)
	getPR(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error)
	listCommits(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error)
	getPRComment(ctx context.Context, owner, repo string, commentID int64) (*github.PullRequestComment, *github.Response, error)
	getIssueComment(ctx context.Context, owner, repo string, commentID int64) (*github.IssueComment, *github.Response, error)
	getMilestone(ctx context.Context, owner, repo string, number int) (*github.Milestone, *github.Response, error)
	listRepositorySecurityAdvisories(ctx context.Context, owner, repo string, opt *github.ListRepositorySecurityAdvisoriesOptions) ([]*github.SecurityAdvisory, *github.Response, error)
	getWorkflowByFileName(ctx context.Context, owner, repo, workflowFileName string) (*github.Workflow, *github.Response, error)
	getWorkflowJobByID(ctx context.Context, owner, repo string, jobID int64) (*github.WorkflowJob, *github.Response, error)
	listWorkflowJobsAttempt(ctx context.Context, owner, repo string, runID, attemptNumber int64, opts *github.ListOptions) (*github.Jobs, *github.Response, error)
	getWorkflowRunByID(ctx context.Context, owner, repo string, runID int64) (*github.WorkflowRun, *github.Response, error)
	getUser(ctx context.Context, user string) (*github.User, *github.Response, error)
	getIssue(ctx context.Context, owner, repo string, number int) (*github.Issue, *github.Response, error)
	getLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error)
	getReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error)
	listLabels(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Label, *github.Response, error)
	getOrganization(ctx context.Context, org string) (*github.Organization, *github.Response, error)
	getGist(ctx context.Context, gistID string) (*github.Gist, *github.Response, error)
	getGistRevision(ctx context.Context, gistID, sha string) (*github.Gist, *github.Response, error)
	getGistComment(ctx context.Context, gistID string, commentID int64) (*github.GistComment, *github.Response, error)
	ListEnvironments(ctx context.Context, owner string, repo string, opts *github.EnvironmentListOptions) (*github.EnvResponse, *github.Response, error)
	GetUserProject(ctx context.Context, username string, projectNumber int) (*github.ProjectV2, *github.Response, error)
	GetOrganizationProject(ctx context.Context, org string, projectNumber int) (*github.ProjectV2, *github.Response, error)
}

type wrapper struct {
	client *github.Client
}

func (w *wrapper) getRepository(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
	return w.client.Repositories.Get(ctx, owner, repo)
}

func (w *wrapper) getContents(ctx context.Context, owner, repo, ref, path string) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error) {
	return w.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
}

func (w *wrapper) getCommit(ctx context.Context, owner, repo, sha string, opts *github.ListOptions) (*github.RepositoryCommit, *github.Response, error) {
	return w.client.Repositories.GetCommit(ctx, owner, repo, sha, opts)
}

func (w *wrapper) compareCommits(ctx context.Context, owner, repo, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error) {
	return w.client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
}

func (w *wrapper) listCommits(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
	return w.client.PullRequests.ListCommits(ctx, owner, repo, number, opts)
}

func (w *wrapper) getPR(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	return w.client.PullRequests.Get(ctx, owner, repo, number)
}

func (w *wrapper) getPRComment(ctx context.Context, owner, repo string, commentID int64) (*github.PullRequestComment, *github.Response, error) {
	return w.client.PullRequests.GetComment(ctx, owner, repo, commentID)
}

func (w *wrapper) getIssueComment(ctx context.Context, owner, repo string, commentID int64) (*github.IssueComment, *github.Response, error) {
	return w.client.Issues.GetComment(ctx, owner, repo, commentID)
}

func (w *wrapper) getMilestone(ctx context.Context, owner, repo string, number int) (*github.Milestone, *github.Response, error) {
	return w.client.Issues.GetMilestone(ctx, owner, repo, number)
}

func (w *wrapper) listRepositorySecurityAdvisories(ctx context.Context, owner, repo string, opt *github.ListRepositorySecurityAdvisoriesOptions) ([]*github.SecurityAdvisory, *github.Response, error) {
	return w.client.SecurityAdvisories.ListRepositorySecurityAdvisories(ctx, owner, repo, opt)
}

func (w *wrapper) getWorkflowByFileName(ctx context.Context, owner, repo, workflowFileName string) (*github.Workflow, *github.Response, error) {
	return w.client.Actions.GetWorkflowByFileName(ctx, owner, repo, workflowFileName)
}

func (w *wrapper) getWorkflowJobByID(ctx context.Context, owner, repo string, jobID int64) (*github.WorkflowJob, *github.Response, error) {
	return w.client.Actions.GetWorkflowJobByID(ctx, owner, repo, jobID)
}

func (w *wrapper) listWorkflowJobsAttempt(ctx context.Context, owner, repo string, runID, attemptNumber int64, opts *github.ListOptions) (*github.Jobs, *github.Response, error) {
	return w.client.Actions.ListWorkflowJobsAttempt(ctx, owner, repo, runID, attemptNumber, opts)
}

func (w *wrapper) getWorkflowRunByID(ctx context.Context, owner, repo string, runID int64) (*github.WorkflowRun, *github.Response, error) {
	return w.client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
}

func (w *wrapper) getUser(ctx context.Context, user string) (*github.User, *github.Response, error) {
	return w.client.Users.Get(ctx, user)
}

func (w *wrapper) getIssue(ctx context.Context, owner, repo string, number int) (*github.Issue, *github.Response, error) {
	return w.client.Issues.Get(ctx, owner, repo, number)
}

func (w *wrapper) getLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
	return w.client.Repositories.GetLatestRelease(ctx, owner, repo)
}

func (w *wrapper) getReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error) {
	return w.client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
}

func (w *wrapper) listLabels(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Label, *github.Response, error) {
	return w.client.Issues.ListLabels(ctx, owner, repo, opts)
}

func (w *wrapper) getOrganization(ctx context.Context, org string) (*github.Organization, *github.Response, error) {
	return w.client.Organizations.Get(ctx, org)
}

func (w *wrapper) getGist(ctx context.Context, gistID string) (*github.Gist, *github.Response, error) {
	return w.client.Gists.Get(ctx, gistID)
}

func (w *wrapper) getGistRevision(ctx context.Context, gistID, sha string) (*github.Gist, *github.Response, error) {
	return w.client.Gists.GetRevision(ctx, gistID, sha)
}

func (w *wrapper) getGistComment(ctx context.Context, gistID string, commentID int64) (*github.GistComment, *github.Response, error) {
	return w.client.Gists.GetComment(ctx, gistID, commentID)
}

func (w *wrapper) ListEnvironments(ctx context.Context, owner string, repo string, opts *github.EnvironmentListOptions) (*github.EnvResponse, *github.Response, error) {
	return w.client.Repositories.ListEnvironments(ctx, owner, repo, opts)
}

func (w *wrapper) GetUserProject(ctx context.Context, owner string, projectNumber int) (*github.ProjectV2, *github.Response, error) {
	return w.client.Projects.GetUserProject(ctx, owner, projectNumber)
}

func (w *wrapper) GetOrganizationProject(ctx context.Context, org string, projectNumber int) (*github.ProjectV2, *github.Response, error) {
	return w.client.Projects.GetOrganizationProject(ctx, org, projectNumber)
}
