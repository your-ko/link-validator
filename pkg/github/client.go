package github

import (
	"context"

	"github.com/google/go-github/v77/github"
)

type Client interface {
	Repositories(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
	GetContents(ctx context.Context, owner, repo, ref, path string) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error)
	GetCommit(ctx context.Context, owner, repo, sha string, opts *github.ListOptions) (*github.RepositoryCommit, *github.Response, error)
	CompareCommits(ctx context.Context, owner, repo, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error)
	GetPR(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error)
	ListCommits(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error)
	GetPRComment(ctx context.Context, owner, repo string, commentID int64) (*github.PullRequestComment, *github.Response, error)
	GetIssueComment(ctx context.Context, owner, repo string, commentID int64) (*github.IssueComment, *github.Response, error)
	GetMilestone(ctx context.Context, owner, repo string, number int) (*github.Milestone, *github.Response, error)
	ListRepositorySecurityAdvisories(ctx context.Context, owner, repo string, opt *github.ListRepositorySecurityAdvisoriesOptions) ([]*github.SecurityAdvisory, *github.Response, error)
	GetWorkflowByFileName(ctx context.Context, owner, repo, workflowFileName string) (*github.Workflow, *github.Response, error)
	GetWorkflowJobByID(ctx context.Context, owner, repo string, jobID int64) (*github.WorkflowJob, *github.Response, error)
	ListWorkflowJobsAttempt(ctx context.Context, owner, repo string, runID, attemptNumber int64, opts *github.ListOptions) (*github.Jobs, *github.Response, error)
	GetWorkflowRunByID(ctx context.Context, owner, repo string, runID int64) (*github.WorkflowRun, *github.Response, error)
}

type wrapper struct {
	client *github.Client
}

func (w *wrapper) Repositories(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
	return w.client.Repositories.Get(ctx, owner, repo)
}

func (w *wrapper) GetContents(ctx context.Context, owner, repo, ref, path string) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error) {
	return w.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
}

func (w *wrapper) GetCommit(ctx context.Context, owner, repo, sha string, opts *github.ListOptions) (*github.RepositoryCommit, *github.Response, error) {
	return w.client.Repositories.GetCommit(ctx, owner, repo, sha, opts)
}

func (w *wrapper) CompareCommits(ctx context.Context, owner, repo, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error) {
	return w.client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
}

func (w *wrapper) ListCommits(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
	return w.client.PullRequests.ListCommits(ctx, owner, repo, number, opts)
}

func (w *wrapper) GetPR(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	return w.client.PullRequests.Get(ctx, owner, repo, number)
}

func (w *wrapper) GetPRComment(ctx context.Context, owner, repo string, commentID int64) (*github.PullRequestComment, *github.Response, error) {
	return w.client.PullRequests.GetComment(ctx, owner, repo, commentID)
}

func (w *wrapper) GetIssueComment(ctx context.Context, owner, repo string, commentID int64) (*github.IssueComment, *github.Response, error) {
	return w.client.Issues.GetComment(ctx, owner, repo, commentID)
}

func (w *wrapper) GetMilestone(ctx context.Context, owner, repo string, number int) (*github.Milestone, *github.Response, error) {
	return w.client.Issues.GetMilestone(ctx, owner, repo, number)
}

func (w *wrapper) ListRepositorySecurityAdvisories(ctx context.Context, owner, repo string, opt *github.ListRepositorySecurityAdvisoriesOptions) ([]*github.SecurityAdvisory, *github.Response, error) {
	return w.client.SecurityAdvisories.ListRepositorySecurityAdvisories(ctx, owner, repo, opt)
}

func (w *wrapper) GetWorkflowByFileName(ctx context.Context, owner, repo, workflowFileName string) (*github.Workflow, *github.Response, error) {
	return w.client.Actions.GetWorkflowByFileName(ctx, owner, repo, workflowFileName)
}

func (w *wrapper) GetWorkflowJobByID(ctx context.Context, owner, repo string, jobID int64) (*github.WorkflowJob, *github.Response, error) {
	return w.client.Actions.GetWorkflowJobByID(ctx, owner, repo, jobID)
}

func (w *wrapper) ListWorkflowJobsAttempt(ctx context.Context, owner, repo string, runID, attemptNumber int64, opts *github.ListOptions) (*github.Jobs, *github.Response, error) {
	return w.client.Actions.ListWorkflowJobsAttempt(ctx, owner, repo, runID, attemptNumber, opts)
}

func (w *wrapper) GetWorkflowRunByID(ctx context.Context, owner, repo string, runID int64) (*github.WorkflowRun, *github.Response, error) {
	w.client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
}
