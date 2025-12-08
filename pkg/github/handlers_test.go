package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"link-validator/pkg/errs"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"
	"time"

	"github.com/google/go-github/v76/github"
)

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
			name:    "test nothing",
			args:    args{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := mockValidator(getTestServer(0, false, ""), "")
			if err := handleNothing(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment); (err != nil) != tt.wantErr {
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

			err := handleRepoExist(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleContents(t *testing.T) {
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
		wantIs           error  // sentinel check via errors.Is; nil => no sentinel check
		wantErrorMessage string // exact error message check; empty => no message check
	}{
		{
			name: "blob file main branch",
			args: args{"your-ko", "link-validator", "main", "README.md", ""},
			fields: fields{
				status:         http.StatusOK,
				body:           "test content",
				base64encoding: true,
			},
		},
		{
			name: "blob file nested directory",
			args: args{"your-ko", "link-validator", "main", "docs/README.md", ""},
			fields: fields{
				status:         http.StatusOK,
				body:           "test content",
				base64encoding: true,
			},
		},
		{
			name: "blob file in tag",
			args: args{"your-ko", "link-validator", "1.0.0", "README.md", ""},
			fields: fields{
				status:         http.StatusOK,
				body:           "test content",
				base64encoding: true,
			},
		},
		{
			name: "refs heads pattern",
			args: args{"your-ko", "link-validator", "refs", "heads/main/README.md", ""},
			fields: fields{
				status:         http.StatusOK,
				body:           "test content",
				base64encoding: true,
			},
		},
		// Error cases
		{
			name: "file not found - 404",
			args: args{"your-ko", "link-validator", "main", "nonexistent.md", ""},
			fields: fields{
				status: http.StatusNotFound,
			},
			wantErr: true,
		},
		{
			name: "server error - 500",
			args: args{"your-ko", "link-validator", "main", "README.md", ""},
			fields: fields{
				status: http.StatusInternalServerError,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := getTestServer(tt.fields.status, tt.fields.base64encoding, tt.fields.body)
			proc := mockValidator(testServer, "")
			t.Cleanup(testServer.Close)

			err := handleContents(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got unexpected error %s", err)
			}
			if !tt.wantErr {
				return
			}

			// Check error type with errors.Is (for wrapped/sentinel errors like errs.ErrNotFound)
			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
				}
			}

			// Check exact error message (for fmt.Errorf() messages)
			if tt.wantErrorMessage != "" {
				if err.Error() != tt.wantErrorMessage {
					t.Fatalf("expected exact error message:\n%q\ngot:\n%q", tt.wantErrorMessage, err.Error())
				}
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
		// Success cases - commits list (empty ref)
		{
			name: "commits list - repository exists",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123, "name": "link-validator"}`,
			},
		},
		{
			name: "specific commit hash",
			args: args{"your-ko", "link-validator", "a96366f66ffacd461de10a1dd561ab5a598e9167", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"sha": "a96366f66ffacd461de10a1dd561ab5a598e9167", "commit": {"message": "test commit"}}`,
			},
		},
		{
			name: "commits list - repository not found",
			args: args{"your-ko", "nonexistent-repo", "", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
		{
			name: "commit not found - 404",
			args: args{"your-ko", "link-validator", "nonexistent-commit-hash", "", ""},
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

			err := handleCommit(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleCompareCommits(t *testing.T) {
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
			name: "compare branches main...dev",
			args: args{"your-ko", "link-validator", "main...dev", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"status": "ahead", "ahead_by": 5, "behind_by": 0, "commits": []}`,
			},
		},
		{
			name: "compare branch to commit hash",
			args: args{"your-ko", "link-validator", "main...a96366f66ffacd461de10a1dd561ab5a598e9167", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"status": "ahead", "ahead_by": 3, "behind_by": 1, "commits": []}`,
			},
		},
		{
			name: "compare commit hashes",
			args: args{"your-ko", "link-validator", "abc123...def456", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"status": "identical", "ahead_by": 0, "behind_by": 0, "commits": []}`,
			},
		},
		{
			name: "compare with tags",
			args: args{"your-ko", "link-validator", "v1.0.0...v1.1.0", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"status": "ahead", "ahead_by": 10, "behind_by": 0, "commits": []}`,
			},
		},
		{
			name: "compare feature branches",
			args: args{"your-ko", "link-validator", "feature-a...feature-b", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"status": "diverged", "ahead_by": 2, "behind_by": 3, "commits": []}`,
			},
		},

		// Validation error cases (fmt.Errorf)
		{
			name: "invalid compare ref - missing dots",
			args: args{"your-ko", "link-validator", "main-dev", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "incorrect GitHub compare URL, expected '/repos/{owner}/{repo}/compare/{basehead}'",
		},
		{
			name: "invalid compare ref - single dot",
			args: args{"your-ko", "link-validator", "main..dev", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "incorrect GitHub compare URL, expected '/repos/{owner}/{repo}/compare/{basehead}'",
		},
		{
			name: "invalid compare ref - empty ref",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "incorrect GitHub compare URL, expected '/repos/{owner}/{repo}/compare/{basehead}'",
		},
		{
			name: "compare ref - only base with empty head",
			args: args{"your-ko", "link-validator", "main...", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"status": "identical", "ahead_by": 0, "behind_by": 0, "commits": []}`,
			},
			wantErr: false, // This passes validation but GitHub API might handle it
		},

		// GitHub API error cases
		{
			name: "repository not found - 404",
			args: args{"your-ko", "nonexistent-repo", "main...dev", "", ""},
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

			err := handleCompareCommits(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handlePull(t *testing.T) {
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
			name: "particular PR",
			args: args{"your-ko", "link-validator", "1", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"number": 1, "title": "Test PR", "state": "open"}`,
			},
		},
		{
			name: "PR with issue comment",
			args: args{"your-ko", "link-validator", "1", "", "issuecomment-123456"},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123456, "body": "This is a comment", "user": {"login": "user"}}`,
			},
		},
		{
			name: "PR with discussion comment",
			args: args{"your-ko", "link-validator", "1", "", "discussion_r123456"},
			fields: fields{
				status: http.StatusOK,
				body:   `{"id": 123456, "body": "Review comment", "user": {"login": "reviewer"}}`,
			},
		},
		{
			name: "invalid PR number - non-numeric",
			args: args{"your-ko", "link-validator", "abc", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: `invalid PR number "abc"`,
		},
		{
			name: "invalid PR number - empty",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: `invalid PR number ""`,
		},
		{
			name: "invalid fragment - bad issue comment ID",
			args: args{"your-ko", "link-validator", "1", "", "issuecomment-abc"},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "invalid comment id: 'issuecomment-abc'",
		},
		{
			name: "invalid fragment - bad discussion comment ID",
			args: args{"your-ko", "link-validator", "1", "", "discussion_rabc"},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "invalid comment id: 'discussion_rabc'",
		},
		{
			name: "unsupported fragment format",
			args: args{"your-ko", "link-validator", "1", "", "unsupported-fragment"},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: "unsupported PR fragment format: 'unsupported-fragment'. Please report a bug",
		},
		{
			name: "PR not found - 404",
			args: args{"your-ko", "link-validator", "1", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
		{
			name: "repository not found",
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

			err := handlePull(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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

func Test_handleMilestone(t *testing.T) {
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
			name: "particular milestone by number",
			args: args{"your-ko", "link-validator", "1", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{"number": 1, "title": "v1.0.0 Release", "state": "open"}`,
			},
		},
		{
			name: "invalid milestone number",
			args: args{"your-ko", "link-validator", "test", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: `invalid milestone number "test"`,
		},
		{
			name: "invalid milestone number - empty",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: `invalid milestone number ""`,
		},
		{
			name: "milestone not found - 404",
			args: args{"your-ko", "link-validator", "1", "", ""},
			fields: fields{
				status: http.StatusNotFound,
				body:   `{"message": "Not Found"}`,
			},
			wantErr: true,
		},
		{
			name: "repository not found",
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

			err := handleMilestone(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment)
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
		p.client = c
		p.corpClient = c
	}
	return p
}
