package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"link-validator/pkg/errs"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"
	"time"

	"github.com/google/go-github/v77/github"
	"github.com/stretchr/testify/mock"
)

var gotGitHubErr *github.ErrorResponse

func Test_handleNothing(t *testing.T) {
	type args struct {
		owner, repo, ref, path, fragment string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "does nothing and makes no client calls",
			args:    args{owner: "test-owner", repo: "test-repo", ref: "test-ref", path: "test-path", fragment: "test-fragment"},
			wantErr: false,
		},
		{
			name:    "does nothing with empty args",
			args:    args{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			err := handleNothing(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if !mockClient.AssertExpectations(t) {
				return
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("handleNothing() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleRepoExist(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	tests := []struct {
		name      string
		args      args
		setupMock func(*mockclient)
		wantErr   error
	}{
		{
			name: "public repository exists",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{
					ID:       github.Ptr(int64(123)),
					Name:     github.Ptr("link-validator"),
					FullName: github.Ptr("your-ko/link-validator"),
					Private:  github.Ptr(false),
					Owner: &github.User{
						Login: github.Ptr("your-ko"),
					},
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "fork repository exists",
			args: args{"contributor", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{
					ID:       github.Ptr(int64(123)),
					Name:     github.Ptr("link-validator"),
					FullName: github.Ptr("contributor/link-validator"),
					Fork:     github.Ptr(true),
					Private:  github.Ptr(false),
					Owner: &github.User{
						Login: github.Ptr("contributor"),
					},
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "contributor", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "repository not found - 404",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "user does not exist - 404",
			args: args{"nonexistent-user", "some-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "nonexistent-user", "some-repo").Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleRepoExist(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}

			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}

			if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}

func Test_handleContents(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	tests := []struct {
		name      string
		setupMock func(*mockclient)
		args      args
		wantErr   error
	}{
		{
			name: "blob file main branch",
			args: args{"your-ko", "link-validator", "main", "README.md", ""},
			setupMock: func(m *mockclient) {
				content := &github.RepositoryContent{
					Name:    github.Ptr("README.md"),
					Path:    github.Ptr("README.md"),
					Content: github.Ptr("test content"),
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getContents(mock.Anything, "your-ko", "link-validator", "main", "README.md").Return(content, nil, resp, nil)
			},
		},
		{
			name: "blob file nested directory",
			args: args{"your-ko", "link-validator", "main", "docs/README.md", ""},
			setupMock: func(m *mockclient) {
				content := &github.RepositoryContent{
					Name:    github.Ptr("README.md"),
					Path:    github.Ptr("/docs"),
					Content: github.Ptr("test content"),
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getContents(mock.Anything, "your-ko", "link-validator", "main", "docs/README.md").Return(content, nil, resp, nil)
			},
		},
		{
			name: "refs heads pattern",
			args: args{"your-ko", "link-validator", "refs", "heads/main/README.md", ""},
			setupMock: func(m *mockclient) {
				content := &github.RepositoryContent{
					Name:    github.Ptr("README.md"),
					Path:    github.Ptr("/"),
					Content: github.Ptr("test content"),
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getContents(mock.Anything, "your-ko", "link-validator", "main", "README.md").Return(content, nil, resp, nil)
			},
		},
		{
			name: "file not found - 404",
			args: args{"your-ko", "link-validator", "main", "nonexistent.md", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getContents(mock.Anything, "your-ko", "link-validator", "main", "nonexistent.md").Return(nil, nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "server error - 500",
			args: args{"your-ko", "link-validator", "main", "README.md", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusInternalServerError},
					Message:  "Server error",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}
				m.EXPECT().getContents(mock.Anything, "your-ko", "link-validator", "main", "README.md").Return(nil, nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusInternalServerError},
				Message:  "Server error",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleContents(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}

			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}

			if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}

func Test_handleCommit(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	tests := []struct {
		name      string
		setupMock func(*mockclient)
		args      args
		wantErr   error
	}{
		{
			name: "commits list - repository exists",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				commit := &github.RepositoryCommit{SHA: github.Ptr("1234567890")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getCommit(mock.Anything, "your-ko", "link-validator", "", (*github.ListOptions)(nil)).Return(commit, resp, nil)
			},
		},
		{
			name: "specific commit hash",
			args: args{"your-ko", "link-validator", "a96366f66ffacd461de10a1dd561ab5a598e9167", "", ""},
			setupMock: func(m *mockclient) {
				commit := &github.RepositoryCommit{SHA: github.Ptr("a96366f66ffacd461de10a1dd561ab5a598e9167")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getCommit(mock.Anything, "your-ko", "link-validator", "a96366f66ffacd461de10a1dd561ab5a598e9167", (*github.ListOptions)(nil)).Return(commit, resp, nil)
			},
		},
		{
			name: "commits list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "a96366", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getCommit(mock.Anything, "your-ko", "nonexistent-repo", "a96366", (*github.ListOptions)(nil)).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{Message: "404 Not found"},
		},
		{
			name: "commit not found - 422",
			args: args{"your-ko", "link-validator", "nonexistent-commit-hash", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusUnprocessableEntity},
					Message:  "No commit found for SHA: nonexistent-commit-hash",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}}
				m.EXPECT().getCommit(mock.Anything, "your-ko", "link-validator", "nonexistent-commit-hash", (*github.ListOptions)(nil)).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{Message: fmt.Sprintf("%v No commit found for SHA: nonexistent-commit-hash", http.StatusUnprocessableEntity)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleCommit(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}

			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}

			if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}

func Test_handleCompareCommits(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	tests := []struct {
		name      string
		setupMock func(*mockclient)
		args      args
		wantErr   error
	}{
		{
			name: "compare branches main...dev",
			args: args{"your-ko", "link-validator", "main...dev", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "main", "dev", (*github.ListOptions)(nil)).Return(compare, resp, nil)
			},
		},
		{
			name: "compare branches dev (no default branch set)",
			args: args{"your-ko", "link-validator", "dev", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{
					ID:            github.Ptr(int64(123)),
					Name:          github.Ptr("link-validator"),
					DefaultBranch: github.Ptr("main"),
				}
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "main", "dev", (*github.ListOptions)(nil)).Return(compare, resp, nil)
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "compare branch to commit hash",
			args: args{"your-ko", "link-validator", "main...a96366f66ffacd461de10a1dd561ab5a598e9167", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "main", "a96366f66ffacd461de10a1dd561ab5a598e9167", (*github.ListOptions)(nil)).Return(compare, resp, nil)
			},
		},
		{
			name: "compare commit hashes",
			args: args{"your-ko", "link-validator", "abc123...def456", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "abc123", "def456", (*github.ListOptions)(nil)).Return(compare, resp, nil)
			},
		},
		{
			name: "compare ref - two dot",
			args: args{"your-ko", "link-validator", "1.15.0..main", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "1.15.0", "main", (*github.ListOptions)(nil)).Return(compare, resp, nil)
			},
		},
		{
			name: "invalid compare ref - empty ref",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{
					ID:   github.Ptr(int64(123)),
					Name: github.Ptr("link-validator"),
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "compare ref - only base with empty head",
			args: args{"your-ko", "link-validator", "main...", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "main", "", (*github.ListOptions)(nil)).Return(compare, resp, nil)
			},
		},
		{
			name: "repository not found - 404",
			args: args{"your-ko", "link-validator", "main...dev", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "main", "dev", (*github.ListOptions)(nil)).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{Message: "404 Not found"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleCompareCommits(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}

			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}

			if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}

func Test_handlePull(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	tests := []struct {
		name      string
		setupMock func(*mockclient)
		args      args
		wantErr   error
	}{
		{
			name: "particular PR",
			args: args{"your-ko", "link-validator", "1", "", ""},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
			},
		},
		{
			name: "PR not found - 404",
			args: args{"your-ko", "link-validator", "1", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name:      "invalid PR number - non-numeric (or empty)",
			args:      args{"your-ko", "link-validator", "abc", "", ""},
			setupMock: func(m *mockclient) {},
			wantErr:   errors.New("invalid PR number '\"abc\"'"),
		},
		{
			name: "PR with issue comment",
			args: args{"your-ko", "link-validator", "1", "", "issuecomment-123456"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				comment := &github.IssueComment{Body: github.Ptr("comment")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
				m.EXPECT().getIssueComment(mock.Anything, "your-ko", "link-validator", int64(123456)).Return(comment, resp, nil)
			},
		},
		{
			name: "PR with not existing issue comment",
			args: args{"your-ko", "link-validator", "1", "", "issuecomment-123456"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
				m.EXPECT().getIssueComment(mock.Anything, "your-ko", "link-validator", int64(123456)).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not found",
			},
		},
		{
			name: "PR with malformed issue comment",
			args: args{"your-ko", "link-validator", "1", "", "issuecomment-aaa"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
			},
			wantErr: errors.New("invalid comment id: 'issuecomment-aaa'"),
		},
		{
			name: "PR with discussion comment",
			args: args{"your-ko", "link-validator", "1", "", "discussion_r123456"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				comment := &github.PullRequestComment{Body: github.Ptr("comment")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
				m.EXPECT().getPRComment(mock.Anything, "your-ko", "link-validator", int64(123456)).Return(comment, resp, nil)
			},
		},
		{
			name: "PR with non existing discussion comment",
			args: args{"your-ko", "link-validator", "1", "", "discussion_r123456"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
				m.EXPECT().getPRComment(mock.Anything, "your-ko", "link-validator", int64(123456)).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not found",
			},
		},
		{
			name: "PR with malformed discussion comment",
			args: args{"your-ko", "link-validator", "1", "", "discussion_raaaaa"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
			},
			wantErr: errors.New("invalid discussion id: 'discussion_raaaaa'"),
		},
		{
			name: "unsupported fragment format",
			args: args{"your-ko", "link-validator", "1", "", "unsupported-fragment"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
			},
			wantErr: errors.New("unsupported PR fragment format: 'unsupported-fragment'. Please report a bug"),
		},
		{
			name: "PR with diff",
			args: args{"your-ko", "link-validator", "1", "", "diff-aaa"},
			setupMock: func(m *mockclient) {
				pr := &github.PullRequest{Title: github.Ptr("great PR")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
			},
		},
		{
			name: "repository not found",
			args: args{"your-ko", "nonexistent-repo", "1", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getPR(mock.Anything, "your-ko", "nonexistent-repo", 1).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handlePull(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}

			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}

			if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}

func Test_handleMilestone(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	tests := []struct {
		name      string
		setupMock func(*mockclient)
		args      args
		wantErr   error
	}{
		{
			name: "particular milestone by number",
			args: args{"your-ko", "link-validator", "1", "", ""},
			setupMock: func(m *mockclient) {
				milestone := &github.Milestone{Title: github.Ptr("great milestone")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getMilestone(mock.Anything, "your-ko", "link-validator", 1).Return(milestone, resp, nil)
			},
		},
		{
			name:      "invalid milestone number",
			args:      args{"your-ko", "link-validator", "test", "", ""},
			setupMock: func(m *mockclient) {},
			wantErr:   fmt.Errorf("invalid milestone number \"test\""),
		},
		{
			name:      "invalid milestone number - empty",
			args:      args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {},
			wantErr:   fmt.Errorf("invalid milestone number \"\""),
		},
		{
			name: "milestone not found - 404",
			args: args{"your-ko", "link-validator", "1", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getMilestone(mock.Anything, "your-ko", "link-validator", 1).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not found",
			},
		},
		{
			name: "repository not found",
			args: args{"your-ko", "nonexistent-repo", "1", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getMilestone(mock.Anything, "your-ko", "nonexistent-repo", 1).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleMilestone(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}
			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}
			if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}

func Test_handleSecurityAdvisories(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		// Success cases
		{
			name: "specific advisory found - GHSA format",
			args: args{"your-ko", "link-validator", "GHSA-xxxx-xxxx-xxxx", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[{"ghsa_id": "GHSA-xxxx-xxxx-xxxx", "summary": "Test security advisory", "severity": "high"}]`,
			},
		},
		{
			name: "empty advisory ID",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[]`,
			},
			wantErr:          true,
			wantErrorMessage: "security advisory ID is required",
		},
		{
			name: "advisory not found - empty list",
			args: args{"your-ko", "link-validator", "GHSA-nonexistent-id", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[]`,
			},
			wantErr:          true,
			wantIs:           errs.ErrNotFound,
			wantErrorMessage: `security advisory "GHSA-nonexistent-id" not found`,
		},
		{
			name: "advisory not found - different advisories in list",
			args: args{"your-ko", "link-validator", "GHSA-missing-xxxx-xxxx", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[{"ghsa_id": "GHSA-1111-2222-3333", "summary": "First advisory"}, {"ghsa_id": "GHSA-4444-5555-6666", "summary": "Second advisory"}]`,
			},
			wantErr:          true,
			wantIs:           errs.ErrNotFound,
			wantErrorMessage: `security advisory "GHSA-missing-xxxx-xxxx" not found`,
		},
		{
			name: "repository not found",
			args: args{"your-ko", "nonexistent-repo", "GHSA-xxxx-xxxx-xxxx", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleSecurityAdvisories(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handleWorkflow(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "actions list - repository exists",
			args: args{"your-ko", "link-validator", "actions", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123, "name": "link-validator"}`,
			},
		},
		{
			name: "specific workflow file",
			args: args{"your-ko", "link-validator", "workflows", "pr.yaml", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 12345, "name": "PR", "path": ".github/workflows/pr.yml", "state": "active"}`,
			},
		},
		{
			name: "workflow badge",
			args: args{"your-ko", "link-validator", "workflows", "build.yml/badge.svg", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 67890, "name": "Build", "path": ".github/workflows/build.yml", "state": "active"}`,
			},
		},
		{
			name: "specific workflow run",
			args: args{"your-ko", "link-validator", "runs", "1234567890", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 1234567890, "run_number": 42, "status": "completed", "conclusion": "success"}`,
			},
		},
		{
			name: "workflow run job",
			args: args{"your-ko", "link-validator", "runs", "1234567890/job/9876543210", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 9876543210, "run_id": 1234567890, "name": "build", "status": "completed"}`,
			},
		},
		{
			name: "workflow run attempt",
			args: args{"your-ko", "link-validator", "runs", "1234567890/attempts/2", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"total_count": 3, "jobs": [{"id": 1, "name": "job1"}, {"id": 2, "name": "job2"}]}`,
			},
		},
		{
			name: "invalid workflow run ID - non-numeric",
			args: args{"your-ko", "link-validator", "runs", "invalid-run-id", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "invalid workflow id: 'invalid-run-id'",
		},
		{
			name: "invalid job ID - non-numeric",
			args: args{"your-ko", "link-validator", "runs", "123456/job/invalid-job", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "invalid job id: '123456/job/invalid-job'",
		},
		{
			name: "invalid attempt ID - non-numeric",
			args: args{"your-ko", "link-validator", "runs", "123456/attempts/invalid-attempt", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "invalid attempt id: '123456/attempts/invalid-attempt'",
		},
		{
			name: "unsupported ref type",
			args: args{"your-ko", "link-validator", "unsupported", "some-path", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "unsupported ref found, please report a bug",
		},
		{
			name: "repository not found - actions list",
			args: args{"your-ko", "nonexistent-repo", "actions", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
		{
			name: "workflow not found - 404",
			args: args{"your-ko", "link-validator", "workflows", "nonexistent.yaml", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleWorkflow(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handleUser(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "existing user",
			args: args{"your-ko", "", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"login": "your-ko", "id": 12345, "type": "User", "name": "Your Ko", "public_repos": 10}`,
			},
		},
		{
			name: "organization user",
			args: args{"github", "", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"login": "github", "id": 9919, "type": "Organization", "name": "GitHub", "public_repos": 100}`,
			},
		},
		{
			name: "user not found - 404",
			args: args{"nonexistent-user", "", "", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleUser(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handleIssue(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "issues list",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"number": 1, "title": "Issues", "state": "open", "user": {"login": "your-ko"}}`,
			},
		},
		{
			name: "specific issue by number",
			args: args{"your-ko", "link-validator", "1", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"number": 1, "title": "Test Issue", "state": "open", "user": {"login": "your-ko"}}`,
			},
		},
		{
			name: "issue with assignees and labels",
			args: args{"your-ko", "link-validator", "123", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"number": 123, "title": "Feature Request", "state": "open", "assignees": [{"login": "assignee1"}], "labels": [{"name": "enhancement"}]}`,
			},
		},
		{
			name: "invalid issue number - non-numeric",
			args: args{"your-ko", "link-validator", "abc", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: `invalid issue number "abc"`,
		},
		{
			name: "issue not found - 404",
			args: args{"your-ko", "link-validator", "999999", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
		{
			name: "repository not found - 404",
			args: args{"your-ko", "nonexistent-repo", "1", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleIssue(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handleReleases(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "latest release exists",
			args: args{"your-ko", "link-validator", "", "latest", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 12345, "tag_name": "v1.0.0", "name": "Release v1.0.0", "draft": false, "prerelease": false}`,
			},
		},
		{
			name: "releases list - repository exists",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123, "name": "link-validator"}`,
			},
		},
		{
			name: "specific release by tag",
			args: args{"your-ko", "link-validator", "tag", "v1.0.0", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 67890, "tag_name": "v1.0.0", "name": "First Release", "draft": false}`,
			},
		},
		{
			name: "download asset - asset exists",
			args: args{"your-ko", "link-validator", "download", "v1.0.0/sbom.spdx.json", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 11111, "tag_name": "v1.0.0", "assets": [{"name": "sbom.spdx.json", "download_count": 100}, {"name": "source.zip", "download_count": 50}]}`,
			},
		},
		{
			name: "download - incorrect path format (missing slash)",
			args: args{"your-ko", "link-validator", "download", "v1.0.0-binary.tar.gz", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "incorrect download path 'v1.0.0-binary.tar.gz' in the release url",
		},
		{
			name: "download - incorrect path format (too many parts)",
			args: args{"your-ko", "link-validator", "download", "v1.0.0/assets/binary.tar.gz", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "incorrect download path 'v1.0.0/assets/binary.tar.gz' in the release url",
		},
		{
			name: "download - asset not found in release",
			args: args{"your-ko", "link-validator", "download", "v1.0.0/nonexistent.zip", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 33333, "tag_name": "v1.0.0", "assets": [{"name": "existing.tar.gz"}, {"name": "another.zip"}]}`,
			},
			wantErr:          true,
			wantIs:           errs.ErrNotFound,
			wantErrorMessage: "asset 'nonexistent.zip' wasn't found in the release assets",
		},
		{
			name: "unexpected release path",
			args: args{"your-ko", "link-validator", "unknown", "some-path", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "unexpected release path 'some-path' found. Please report a bug",
		},
		{
			name: "latest release not found - 404",
			args: args{"your-ko", "link-validator", "", "latest", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
		{
			name: "repository not found - releases list",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleReleases(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handleLabel(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "label found - multiple labels",
			args: args{"your-ko", "link-validator", "enhancement", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[{"name": "bug", "color": "d73a4a"}, {"name": "enhancement", "color": "a2eeef", "description": "New feature or request"}, {"name": "help wanted", "color": "008672"}]`,
			},
		},
		{
			name: "label not found - empty labels list",
			args: args{"your-ko", "link-validator", "nonexistent", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[]`,
			},
			wantErr:          true,
			wantIs:           errs.ErrNotFound,
			wantErrorMessage: "label 'nonexistent' not found",
		},
		{
			name: "label not found - different labels exist",
			args: args{"your-ko", "link-validator", "missing-label", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[{"name": "bug", "color": "d73a4a"}, {"name": "enhancement", "color": "a2eeef"}, {"name": "documentation", "color": "0075ca"}]`,
			},
			wantErr:          true,
			wantIs:           errs.ErrNotFound,
			wantErrorMessage: "label 'missing-label' not found",
		},
		{
			name: "label not found - case sensitive mismatch",
			args: args{"your-ko", "link-validator", "Bug", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `[{"name": "bug", "color": "d73a4a"}, {"name": "enhancement", "color": "a2eeef"}]`,
			},
			wantErr:          true,
			wantIs:           errs.ErrNotFound,
			wantErrorMessage: "label 'Bug' not found",
		},
		{
			name: "repository not found - 404",
			args: args{"your-ko", "nonexistent-repo", "bug", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleLabel(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handleWiki(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "repository with wiki enabled",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123, "name": "link-validator", "has_wiki": true, "owner": {"login": "your-ko"}}`,
			},
		},
		{
			name: "repository exists but wiki disabled",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123, "name": "link-validator", "has_wiki": false, "owner": {"login": "your-ko"}}`,
			},
			wantErr:          true,
			wantIs:           errs.ErrNotFound,
			wantErrorMessage: "wiki is not enabled for repository your-ko/link-validator",
		},
		{
			name: "repository not found - 404",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleWiki(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handleOrgExist(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "organization exists",
			args: args{"github", "", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"login": "github", "id": 9919, "type": "Organization", "name": "GitHub", "company": null, "blog": "https://github.com/about"}`,
			},
		},
		{
			name: "empty owner - should return nil",
			args: args{"", "", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`, // This won't be called since owner is empty
			},
		},
		{
			name: "organization not found - 404",
			args: args{"nonexistent-org", "", "", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleOrgExist(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func Test_handlePackages(t *testing.T) {
	type args struct {
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
	}
	type fields struct {
		status         int
		body           string
		base64encoding bool
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		wantIs           error
		wantErrorMessage string
	}{
		{
			name: "public repository exists",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123, "name": "link-validator", "full_name": "your-ko/link-validator", "private": false, "owner": {"login": "your-ko"}}`,
			},
		},
		{
			name: "fork repository exists",
			args: args{"contributor", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 999, "name": "link-validator", "full_name": "contributor/link-validator", "fork": true, "private": false, "owner": {"login": "contributor"}}`,
			},
		},
		{
			name: "repository not found - 404",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
		{
			name: "user does not exist - 404",
			args: args{"nonexistent-user", "some-repo", "", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handlePackages(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
			}
		})
	}
}

func getTestServer(httpStatus int, base64enc bool, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		//if tt.fields.loc != "" {
		//	res.Header().Set("Location", tt.fields.loc)
		//}
		res.WriteHeader(httpStatus)

		if base64enc {
			_ = json.NewEncoder(res).Encode(&githubContent{
				Type:     "file",
				Encoding: "base64",
				Content:  base64.StdEncoding.EncodeToString([]byte(body)),
			})
		} else {
			res.Header().Set("Content-Type", "application/json")
			_, _ = res.Write([]byte(body))
		}
	}))
}

type githubContent struct {
	Type     string `json:"type"`     // "file" or "dir"
	Encoding string `json:"encoding"` // "base64" for file
	Content  string `json:"content"`  // base64-encoded file body
}

// mockValidator creates a validator instance with mock GitHub clients
func mockValidator(ts *httptest.Server, corp string) *LinkProcessor {
	p, _ := New(corp, "", "", time.Second)

	if ts != nil {
		base, _ := neturl.Parse(ts.URL + "/")
		c := github.NewClient(ts.Client())
		c.BaseURL = base
		c.UploadURL = base
		p.client = &wrapper{c}
		p.corpClient = &wrapper{c}
	}
	return p
}
