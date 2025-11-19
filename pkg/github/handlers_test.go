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
			name: "blob file in commit",
			args: args{"your-ko", "link-validator", "83e43288254d0f36e723ef2cf3328b8b77836560", "README.md", ""},
			fields: fields{
				status:         http.StatusOK,
				body:           "test content",
				base64encoding: true,
			},
		},
		{
			name: "tree directory",
			args: args{"your-ko", "link-validator", "main", "cmd", ""},
			fields: fields{
				status: http.StatusOK,
				body:   "[]",
			},
		},
		{
			name: "refs heads pattern",
			args: args{"your-ko", "link-validator", "refs", "heads/main/Dockerfile", ""},
			fields: fields{
				status:         http.StatusOK,
				body:           "FROM alpine",
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
			wantIs:  nil, // handleContents doesn't use mapGHError, so we get raw GitHub API error
		},
		{
			name: "server error - 500",
			args: args{"your-ko", "link-validator", "main", "README.md", ""},
			fields: fields{
				status: http.StatusInternalServerError,
			},
			wantErr: true,
			wantIs:  nil,
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
			if err := handleCommit(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleCommit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleCompareCommits(t *testing.T) {
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
			if err := handleCompareCommits(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleCompareCommits() error = %v, wantErr %v", err, tt.wantErr)
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

func Test_handleMilestone(t *testing.T) {
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
			if err := handleMilestone(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleMilestone() error = %v, wantErr %v", err, tt.wantErr)
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

func Test_handlePull(t *testing.T) {
	type args struct {
		ctx      context.Context
		c        *github.Client
		owner    string
		repo     string
		ref      string
		in5      string
		fragment string
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
			if err := handlePull(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.fragment); (err != nil) != tt.wantErr {
				t.Errorf("handlePull() error = %v, wantErr %v", err, tt.wantErr)
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

func Test_handleSecurityAdvisories(t *testing.T) {
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
			if err := handleSecurityAdvisories(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.in5, tt.args.in6); (err != nil) != tt.wantErr {
				t.Errorf("handleSecurityAdvisories() error = %v, wantErr %v", err, tt.wantErr)
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

func Test_handleWorkflow(t *testing.T) {
	type args struct {
		ctx      context.Context
		c        *github.Client
		owner    string
		repo     string
		ref      string
		path     string
		fragment string
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
			if err := handleWorkflow(tt.args.ctx, tt.args.c, tt.args.owner, tt.args.repo, tt.args.ref, tt.args.path, tt.args.fragment); (err != nil) != tt.wantErr {
				t.Errorf("handleWorkflow() error = %v, wantErr %v", err, tt.wantErr)
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
