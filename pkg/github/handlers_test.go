package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v81/github"
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
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getCommit(mock.Anything, "your-ko", "link-validator", "", (*github.ListOptions)(nil)).Return(commit, resp, nil)
			},
		},
		{
			name: "specific commit hash",
			args: args{"your-ko", "link-validator", "a96366f66ffacd461de10a1dd561ab5a598e9167", "", ""},
			setupMock: func(m *mockclient) {
				commit := &github.RepositoryCommit{SHA: github.Ptr("a96366f66ffacd461de10a1dd561ab5a598e9167")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getCommit(mock.Anything, "your-ko", "link-validator", "a96366f66ffacd461de10a1dd561ab5a598e9167", (*github.ListOptions)(nil)).Return(commit, resp, nil)
			},
		},
		{
			name: "commits list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "commits", "main", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "main", "a96366f66ffacd461de10a1dd561ab5a598e9167", (*github.ListOptions)(nil)).Return(compare, resp, nil)
			},
		},
		{
			name: "compare commit hashes",
			args: args{"your-ko", "link-validator", "abc123...def456", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().compareCommits(mock.Anything, "your-ko", "link-validator", "abc123", "def456", (*github.ListOptions)(nil)).Return(compare, resp, nil)
			},
		},
		{
			name: "compare ref - two dot",
			args: args{"your-ko", "link-validator", "1.15.0..main", "", ""},
			setupMock: func(m *mockclient) {
				compare := &github.CommitsComparison{}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
			},
		},
		{
			name: "PR not found",
			args: args{"your-ko", "link-validator", "1", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getPR(mock.Anything, "your-ko", "link-validator", 1).Return(pr, resp, nil)
			},
		},
		{
			name: "commits list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "1", "commits", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
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
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getMilestone(mock.Anything, "your-ko", "link-validator", 1).Return(milestone, resp, nil)
			},
		},
		{
			name: "invalid milestone number",
			args: args{"your-ko", "link-validator", "test", "", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: fmt.Errorf("invalid milestone number \"test\""),
		},
		{
			name: "milestone not found",
			args: args{"your-ko", "link-validator", "1", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getMilestone(mock.Anything, "your-ko", "link-validator", 1).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not found",
			},
		},
		{
			name: "milestones list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
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
	tests := []struct {
		name      string
		setupMock func(*mockclient)
		args      args
		wantErr   error
	}{
		{
			name: "specific advisory found - GHSA format",
			args: args{"your-ko", "link-validator", "GHSA-1234-5678-9012", "", ""},
			setupMock: func(m *mockclient) {
				sa := []*github.SecurityAdvisory{{
					GHSAID:  github.Ptr("GHSA-1234-5678-9012"),
					Summary: github.Ptr("be secure"),
				}}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listRepositorySecurityAdvisories(mock.Anything, "your-ko", "link-validator", (*github.ListRepositorySecurityAdvisoriesOptions)(nil)).Return(sa, resp, nil)
			},
		},
		{
			name:      "empty advisory ID",
			args:      args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {},
			wantErr:   errors.New("security advisory ID is required"),
		},
		{
			name: "advisory not found - empty list",
			args: args{"your-ko", "link-validator", "GHSA-nonexistent-id", "", ""},
			setupMock: func(m *mockclient) {
				sa := []*github.SecurityAdvisory{{
					GHSAID:  github.Ptr("GHSA-1234-5678-9012"),
					Summary: github.Ptr("be secure"),
				}}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listRepositorySecurityAdvisories(mock.Anything, "your-ko", "link-validator", (*github.ListRepositorySecurityAdvisoriesOptions)(nil)).Return(sa, resp, nil)
			},
			wantErr: errors.New("security advisory \"GHSA-nonexistent-id\" not found"),
		},
		{
			name: "advisory not found - different advisories in list",
			args: args{"your-ko", "link-validator", "GHSA-1234-5678-9012", "", ""},
			setupMock: func(m *mockclient) {
				sa := []*github.SecurityAdvisory{{
					GHSAID:  github.Ptr("GHSA-0000-0000-0000"),
					Summary: github.Ptr("be secure"),
				}}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listRepositorySecurityAdvisories(mock.Anything, "your-ko", "link-validator", (*github.ListRepositorySecurityAdvisoriesOptions)(nil)).Return(sa, resp, nil)
			},
			wantErr: errors.New("security advisory \"GHSA-1234-5678-9012\" not found"),
		},
		{
			name:      "advisories list - repository not found",
			args:      args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {},
			wantErr:   errors.New("security advisory ID is required"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleSecurityAdvisories(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleWorkflow(t *testing.T) {
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
			name: "actions list - repository exists",
			args: args{"your-ko", "link-validator", "actions", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{
					Name: github.Ptr("link-validator"),
					Owner: &github.User{
						Login: github.Ptr("your-ko"),
					},
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "specific workflow file",
			args: args{"your-ko", "link-validator", "workflows", "pr.yaml", ""},
			setupMock: func(m *mockclient) {
				workflow := &github.Workflow{Name: github.Ptr("pr.yaml")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getWorkflowByFileName(mock.Anything, "your-ko", "link-validator", "pr.yaml").Return(workflow, resp, nil)
			},
		},
		{
			name: "specific workflow file with badge",
			args: args{"your-ko", "link-validator", "workflows", "pr.yaml/badge.svg", ""},
			setupMock: func(m *mockclient) {
				workflow := &github.Workflow{Name: github.Ptr("pr.yaml")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getWorkflowByFileName(mock.Anything, "your-ko", "link-validator", "pr.yaml").Return(workflow, resp, nil)
			},
		},
		{
			name: "invalid workflow run ID - non-numeric",
			args: args{"your-ko", "link-validator", "runs", "invalid-run-id", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("invalid workflow id: 'invalid-run-id'"),
		},
		{
			name: "workflow not found - 404",
			args: args{"your-ko", "link-validator", "workflows", "nonexistent.yaml", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getWorkflowByFileName(mock.Anything, "your-ko", "link-validator", "nonexistent.yaml").Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "specific workflow run",
			args: args{"your-ko", "link-validator", "runs", "1234567890", ""},
			setupMock: func(m *mockclient) {
				workflow := &github.WorkflowRun{Name: github.Ptr("pr.yaml")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getWorkflowRunByID(mock.Anything, "your-ko", "link-validator", int64(1234567890)).Return(workflow, resp, nil)
			},
		},
		{
			name: "specific not-existing workflow run",
			args: args{"your-ko", "link-validator", "runs", "1234567890", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getWorkflowRunByID(mock.Anything, "your-ko", "link-validator", int64(1234567890)).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "workflow run job by id",
			args: args{"your-ko", "link-validator", "runs", "1234567890/job/9876543210", ""},
			setupMock: func(m *mockclient) {
				job := &github.WorkflowJob{Name: github.Ptr("pr.yaml")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getWorkflowJobByID(mock.Anything, "your-ko", "link-validator", int64(9876543210)).Return(job, resp, nil)
			},
		},
		{
			name: "malformed workflow run job id",
			args: args{"your-ko", "link-validator", "runs", "1234567890/job/qwerty", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("invalid job id: '1234567890/job/qwerty'"),
		},
		{
			name: "workflow run job by id not found",
			args: args{"your-ko", "link-validator", "runs", "1234567890/job/9876543210", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getWorkflowJobByID(mock.Anything, "your-ko", "link-validator", int64(9876543210)).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "workflow run attempt",
			args: args{"your-ko", "link-validator", "runs", "1234567890/attempts/2", ""},
			setupMock: func(m *mockclient) {
				jobs := &github.Jobs{TotalCount: github.Ptr(5)}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listWorkflowJobsAttempt(mock.Anything, "your-ko", "link-validator", int64(1234567890), int64(2), (*github.ListOptions)(nil)).Return(jobs, resp, nil)
			},
		},
		{
			name: "workflow run not existing attempt",
			args: args{"your-ko", "link-validator", "runs", "1234567890/attempts/2", ""},
			setupMock: func(m *mockclient) {
				jobs := &github.Jobs{TotalCount: github.Ptr(1)}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listWorkflowJobsAttempt(mock.Anything, "your-ko", "link-validator", int64(1234567890), int64(2), (*github.ListOptions)(nil)).Return(jobs, resp, nil)
			},
			wantErr: errors.New("job attempt '2' not found"),
		},
		{
			name: "invalid attempt ID - non-numeric",
			args: args{"your-ko", "link-validator", "runs", "123456/attempts/invalid-attempt", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("invalid attempt id: '123456/attempts/invalid-attempt'"),
		},
		{
			name: "unsupported ref type",
			args: args{"your-ko", "link-validator", "unsupported", "some-path", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("unsupported ref found, please report a bug"),
		},
		{
			name: "workflows - repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleWorkflow(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleUser(t *testing.T) {
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
			name: "existing user",
			args: args{"your-ko", "", "", "", ""},
			setupMock: func(m *mockclient) {
				user := &github.User{Name: github.Ptr("your-ko")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getUser(mock.Anything, "your-ko").Return(user, resp, nil)
			},
		},
		{
			name: "user not found - 404",
			args: args{"nonexistent-user", "", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getUser(mock.Anything, "nonexistent-user").Return(nil, resp, err)
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

			err := handleUser(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleIssue(t *testing.T) {
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
			name: "issues list",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "specific issue by number",
			args: args{"your-ko", "link-validator", "1", "", ""},
			setupMock: func(m *mockclient) {
				issue := &github.Issue{Title: github.Ptr("super issue")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getIssue(mock.Anything, "your-ko", "link-validator", 1).Return(issue, resp, nil)
			},
		},
		{
			name: "invalid issue number - non-numeric",
			args: args{"your-ko", "link-validator", "abc", "", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("invalid issue number \"abc\""),
		},
		{
			name: "issue not found - 404",
			args: args{"your-ko", "link-validator", "999999", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getIssue(mock.Anything, "your-ko", "link-validator", 999999).Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "issues list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleIssue(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleReleases(t *testing.T) {
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
			name: "latest release exists",
			args: args{"your-ko", "link-validator", "", "latest", ""},
			setupMock: func(m *mockclient) {
				release := &github.RepositoryRelease{Name: github.Ptr("cool release")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getLatestRelease(mock.Anything, "your-ko", "link-validator").Return(release, resp, nil)
			},
		},
		{
			name: "releases list - repository exists",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "specific release by tag",
			args: args{"your-ko", "link-validator", "tag", "v1.0.0", ""},
			setupMock: func(m *mockclient) {
				release := &github.RepositoryRelease{Name: github.Ptr("cool release")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getReleaseByTag(mock.Anything, "your-ko", "link-validator", "v1.0.0").Return(release, resp, nil)
			},
		},
		{
			name: "specific release by tag not found",
			args: args{"your-ko", "link-validator", "tag", "v1.0.0", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getReleaseByTag(mock.Anything, "your-ko", "link-validator", "v1.0.0").Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "download asset - asset exists",
			args: args{"your-ko", "link-validator", "download", "v1.0.0/sbom.spdx.json", ""},
			setupMock: func(m *mockclient) {
				release := &github.RepositoryRelease{
					Name:   github.Ptr("cool release"),
					Assets: []*github.ReleaseAsset{{Name: github.Ptr("sbom.spdx.json")}},
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getReleaseByTag(mock.Anything, "your-ko", "link-validator", "v1.0.0").Return(release, resp, nil)
			},
		},
		{
			name: "download - incorrect path format (missing slash)",
			args: args{"your-ko", "link-validator", "download", "v1.0.0-binary.tar.gz", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("incorrect download path 'v1.0.0-binary.tar.gz' in the release url"),
		},
		{
			name: "download - incorrect path format (too many parts)",
			args: args{"your-ko", "link-validator", "download", "v1.0.0/assets/binary.tar.gz", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("incorrect download path 'v1.0.0/assets/binary.tar.gz' in the release url"),
		},
		{
			name: "download - asset not found in release",
			args: args{"your-ko", "link-validator", "download", "v1.0.0/nonexistent.zip", ""},
			setupMock: func(m *mockclient) {
				release := &github.RepositoryRelease{
					Name:   github.Ptr("cool release"),
					Assets: []*github.ReleaseAsset{{Name: github.Ptr("sbom.spdx.json")}},
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getReleaseByTag(mock.Anything, "your-ko", "link-validator", "v1.0.0").Return(release, resp, nil)
			},
			wantErr: errors.New("asset 'nonexistent.zip' wasn't found in the release assets"),
		},
		{
			name: "unexpected release path",
			args: args{"your-ko", "link-validator", "unknown", "some-path", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("unexpected release path 'some-path' found. Please report a bug"),
		},
		{
			name: "latest release not found - 404",
			args: args{"your-ko", "link-validator", "", "latest", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().getLatestRelease(mock.Anything, "your-ko", "link-validator").Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
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
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleReleases(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleLabel(t *testing.T) {
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
			name: "label found in the list",
			args: args{"your-ko", "link-validator", "enhancement", "", ""},
			setupMock: func(m *mockclient) {
				label := []*github.Label{{Name: github.Ptr("enhancement")}}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listLabels(mock.Anything, "your-ko", "link-validator", (*github.ListOptions)(nil)).Return(label, resp, nil)
			},
		},
		{
			name: "label not found - empty labels list",
			args: args{"your-ko", "link-validator", "nonexistent", "", ""},
			setupMock: func(m *mockclient) {
				var label []*github.Label
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listLabels(mock.Anything, "your-ko", "link-validator", (*github.ListOptions)(nil)).Return(label, resp, nil)
			},
			wantErr: errors.New("label 'nonexistent' not found"),
		},
		{
			name: "label not found - case sensitive mismatch",
			args: args{"your-ko", "link-validator", "Bug", "", ""},
			setupMock: func(m *mockclient) {
				label := []*github.Label{{Name: github.Ptr("bug")}}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listLabels(mock.Anything, "your-ko", "link-validator", (*github.ListOptions)(nil)).Return(label, resp, nil)
			},
			wantErr: errors.New("label 'Bug' not found"),
		},
		{
			name: "labels list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleLabel(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleWiki(t *testing.T) {
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
			name: "repository with wiki enabled",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{
					Name:    github.Ptr("link-validator"),
					HasWiki: github.Ptr(true),
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "repository exists but wiki disabled",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{
					Name:    github.Ptr("link-validator"),
					HasWiki: github.Ptr(false),
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("wiki is not enabled for repository your-ko/link-validator"),
		},
		{
			name: "wiki: repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleWiki(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

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

func Test_handleOrgExist(t *testing.T) {
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
			name: "organization exists",
			args: args{"your-ko", "", "", "", ""},
			setupMock: func(m *mockclient) {
				org := &github.Organization{
					Name: github.Ptr("your-ko"),
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getOrganization(mock.Anything, "your-ko").Return(org, resp, nil)
			},
		},
		{
			name:      "empty owner - should return nil",
			args:      args{"", "", "", "", ""},
			setupMock: func(m *mockclient) {},
		},
		{
			name: "organization not found - 404",
			args: args{"nonexistent-org", "", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getOrganization(mock.Anything, "nonexistent-org").Return(nil, resp, err)
			},
			wantErr: errors.New("org 'nonexistent-org' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleOrgExist(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

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

func Test_handlePackages(t *testing.T) {
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
			name: "packages list - repository exists",
			args: args{"your-ko", "link-validator", "", "", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
		},
		{
			name: "packages list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
		},
		{
			name: "specific package - container package found as user package",
			args: args{"your-ko", "link-validator", "container", "link-validator", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				repoResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, repoResp, nil)

				pkg := &github.Package{Name: github.Ptr("link-validator")}
				pkgResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getUserPackage(mock.Anything, "your-ko", "container", "link-validator").Return(pkg, pkgResp, nil)
			},
		},
		{
			name: "specific package - container package found as org package",
			args: args{"your-ko", "link-validator", "container", "link-validator", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				repoResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, repoResp, nil)

				userErr := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				userResp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getUserPackage(mock.Anything, "your-ko", "container", "link-validator").Return(nil, userResp, userErr)

				// Org package found
				pkg := &github.Package{Name: github.Ptr("link-validator")}
				orgResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getOrgPackage(mock.Anything, "your-ko", "container", "link-validator").Return(pkg, orgResp, nil)
			},
		},
		{
			name: "specific package with version - container package found",
			args: args{"your-ko", "link-validator", "container", "link-validator/617266022", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				repoResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, repoResp, nil)

				// User package found (note: only package name used, version ignored for API call)
				pkg := &github.Package{Name: github.Ptr("link-validator")}
				pkgResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getUserPackage(mock.Anything, "your-ko", "container", "link-validator").Return(pkg, pkgResp, nil)
			},
		},
		{
			name: "specific package - package not found",
			args: args{"your-ko", "link-validator", "container", "nonexistent-package", ""},
			setupMock: func(m *mockclient) {
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				repoResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, repoResp, nil)

				userErr := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Package not found",
				}
				userResp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getUserPackage(mock.Anything, "your-ko", "container", "nonexistent-package").Return(nil, userResp, userErr)

				orgErr := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Package not found",
				}
				orgResp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getOrgPackage(mock.Anything, "your-ko", "container", "nonexistent-package").Return(nil, orgResp, orgErr)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Package not found",
			},
		},
		{
			name: "pkgs URL without package name",
			args: args{"your-ko", "link-validator", "container", "", ""},
			setupMock: func(m *mockclient) {
				// Repository exists
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				repoResp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, repoResp, nil)
			},
			wantErr: errors.New("package name is required for /pkgs URLs"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handlePackages(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)

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
			var gotGitHubErr *github.ErrorResponse
			if errors.As(tt.wantErr, &gotGitHubErr) && !errors.As(err, &gotGitHubErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}

func Test_handleGist(t *testing.T) {
	type args struct {
		owner    string
		gist     string
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
			name: "existing gist",
			args: args{"your-ko", "gist123", "", "", ""},
			setupMock: func(m *mockclient) {
				gist := &github.Gist{ID: github.Ptr("your-ko")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getGist(mock.Anything, "gist123").Return(gist, resp, nil)
			},
		},
		{
			name: "gist with revision",
			args: args{"your-ko", "gist123", "12345", "", ""},
			setupMock: func(m *mockclient) {
				gist := &github.Gist{ID: github.Ptr("your-ko")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getGistRevision(mock.Anything, "gist123", "12345").Return(gist, resp, nil)
			},
		},
		{
			name: "gist with comment fragment",
			args: args{"your-ko", "gist123", "", "", "gistcomment-12345"},
			setupMock: func(m *mockclient) {
				comment := &github.GistComment{ID: github.Ptr(int64(12345))}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getGistComment(mock.Anything, "gist123", int64(12345)).Return(comment, resp, nil)
			},
		},
		{
			name: "gist not found - 404",
			args: args{"your-ko", "nonexistent", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getGist(mock.Anything, "nonexistent").Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name:      "invalid comment ID in fragment",
			args:      args{"your-ko", "gist123", "", "", "gistcomment-invalid"},
			setupMock: func(m *mockclient) {},
			wantErr:   errors.New("invalid gist comment id: 'gistcomment-invalid'"),
		},
		{
			name: "comment not found - 404",
			args: args{"your-ko", "gist123", "", "", "gistcomment-99999"},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getGistComment(mock.Anything, "gist123", int64(99999)).Return(nil, resp, err)
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

			err := handleGist(context.Background(), mockClient, tt.args.owner, tt.args.gist, tt.args.ref, tt.args.path, tt.args.fragment)

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

func Test_handleEnvironments(t *testing.T) {
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
			name: "env exists",
			args: args{"your-ko", "link-validator", "12345", "", ""},
			setupMock: func(m *mockclient) {
				envs := &github.EnvResponse{
					TotalCount: github.Ptr(12345),
					Environments: []*github.Environment{{
						ID:   github.Ptr(int64(12345)),
						Name: github.Ptr("test"),
					}},
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listEnvironments(mock.Anything, "your-ko", "link-validator", (*github.EnvironmentListOptions)(nil)).Return(envs, resp, nil)
			},
		},
		{
			name: "env not found",
			args: args{"your-ko", "link-validator", "09876", "", ""},
			setupMock: func(m *mockclient) {
				envs := &github.EnvResponse{
					TotalCount: github.Ptr(12345),
					Environments: []*github.Environment{{
						ID:   github.Ptr(int64(12345)),
						Name: github.Ptr("test"),
					}},
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
				m.EXPECT().listEnvironments(mock.Anything, "your-ko", "link-validator", (*github.EnvironmentListOptions)(nil)).Return(envs, resp, nil)
			},
			wantErr: errors.New("environment with id:09876 not found"),
		},
		{
			name: "env id is malformed",
			args: args{"your-ko", "link-validator", "qwerty", "", ""},
			setupMock: func(m *mockclient) {
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				repo := &github.Repository{Name: github.Ptr("link-validator")}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "link-validator").Return(repo, resp, nil)
			},
			wantErr: errors.New("invalid environment id: 'qwerty'"),
		},
		{
			name: "repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getRepository(mock.Anything, "your-ko", "nonexistent-repo").Return(nil, resp, err)
			},
			wantErr: errors.New("repository 'nonexistent-repo' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleEnvironments(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleTeams(t *testing.T) {
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
			name: "team exists",
			args: args{"mycorp", "", "sre", "", ""},
			setupMock: func(m *mockclient) {
				team := &github.Team{Name: github.Ptr("sre")}
				org := &github.Organization{Name: github.Ptr("mycorp")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getOrganization(mock.Anything, "mycorp").Return(org, resp, nil)
				m.EXPECT().getTeamBySlug(mock.Anything, "mycorp", "sre").Return(team, resp, nil)
			},
		},
		{
			name: "team is not specified",
			args: args{"mycorp", "", "", "", ""},
			setupMock: func(m *mockclient) {
				org := &github.Organization{Name: github.Ptr("mycorp")}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
				m.EXPECT().getOrganization(mock.Anything, "mycorp").Return(org, resp, nil)
			},
		},
		{
			name: "team not found",
			args: args{"mycorp", "", "sre", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				org := &github.Organization{Name: github.Ptr("mycorp")}
				m.EXPECT().getOrganization(mock.Anything, "mycorp").Return(org, resp, nil)
				m.EXPECT().getTeamBySlug(mock.Anything, "mycorp", "sre").Return(nil, resp, err)
			},
			wantErr: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
		},
		{
			name: "org is not found",
			args: args{"mycorp", "", "sre", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getOrganization(mock.Anything, "mycorp").Return(nil, resp, err)
			},
			wantErr: errors.New("org 'mycorp' not found"),
		},
		{
			name: "org not found",
			args: args{"mycorp", "", "", "", ""},
			setupMock: func(m *mockclient) {
				err := &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not found",
				}
				resp := &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
				m.EXPECT().getOrganization(mock.Anything, "mycorp").Return(nil, resp, err)
			},
			wantErr: errors.New("org 'mycorp' not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockclient(t)
			tt.setupMock(mockClient)

			err := handleTeams(context.Background(), mockClient, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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
