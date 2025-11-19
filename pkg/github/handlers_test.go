package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v74/github"
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
		path           string
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
		owner string
		repo  string
		ref   string
		in5   string
		in6   string
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

			err := handleCommit(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6)
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
		owner string
		repo  string
		ref   string
		in5   string
		in6   string
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

			err := handleCompareCommits(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6)
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
		in5      string
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
			wantErrorMessage: `invalid PR number "abc": strconv.Atoi: parsing "abc": invalid syntax`,
		},
		{
			name: "invalid PR number - empty",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: `invalid PR number "": strconv.Atoi: parsing "": invalid syntax`,
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

			err := handlePull(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.fragment)
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
		owner string
		repo  string
		ref   string
		in5   string
		in6   string
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
			wantErrorMessage: `invalid milestone number "test": strconv.Atoi: parsing "test": invalid syntax`,
		},
		{
			name: "invalid milestone number - empty",
			args: args{"your-ko", "link-validator", "", "", ""},
			fields: fields{
				status: http.StatusOK,
				body:   `{}`,
			},
			wantErr:          true,
			wantErrorMessage: `invalid milestone number "": strconv.Atoi: parsing "": invalid syntax`,
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

			err := handleMilestone(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6)
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
		owner string
		repo  string
		ref   string
		in5   string
		in6   string
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

			err := handleSecurityAdvisories(context.Background(), proc.client, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6)
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
			wantErrorMessage: "unsupported ref found",
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

func Test_handleIssue(t *testing.T) {
	type args struct {
		ctx   context.Context
		c     *github.Client
		owner string
		repo  string
		ref   string
		in5   string
		in6   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleIssue(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleIssue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleLabel(t *testing.T) {
	type args struct {
		ctx   context.Context
		c     *github.Client
		owner string
		repo  string
		ref   string
		in5   string
		in6   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleLabel(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleLabel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleOrgExist(t *testing.T) {
	type args struct {
		ctx   context.Context
		c     *github.Client
		owner string
		in3   string
		in4   string
		in5   string
		in6   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleOrgExist(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.in3, tt.args.in4, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleOrgExist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handlePackages(t *testing.T) {
	type args struct {
		ctx         context.Context
		c           *github.Client
		owner       string
		repo        string
		packageType string
		packageName string
		fragment    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handlePackages(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.packageType, tt.args.packageName, tt.args.fragment); (err != nil) != tt.wantErr {
				t.Errorf("handlePackages() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleReleases(t *testing.T) {
	type args struct {
		ctx   context.Context
		c     *github.Client
		owner string
		repo  string
		ref   string
		path  string
		in6   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleReleases(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleReleases() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleRepoExist(t *testing.T) {
	type args struct {
		ctx   context.Context
		c     *github.Client
		owner string
		repo  string
		in4   string
		in5   string
		in6   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleRepoExist(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.in4, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleRepoExist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleUser(t *testing.T) {
	type args struct {
		ctx   context.Context
		c     *github.Client
		owner string
		in3   string
		in4   string
		in5   string
		in6   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleUser(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.in3, tt.args.in4, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleWiki(t *testing.T) {
	type args struct {
		ctx   context.Context
		c     *github.Client
		owner string
		repo  string
		in4   string
		in5   string
		in6   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleWiki(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.in4, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleWiki() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_mapGHError(t *testing.T) {
	type args struct {
		url string
		err error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := mapGHError(tt.args.url, tt.args.err); (err != nil) != tt.wantErr {
				t.Errorf("mapGHError() error = %v, wantErr %v", err, tt.wantErr)
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
