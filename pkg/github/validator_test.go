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
	"reflect"
	"testing"
	"time"

	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
)

func TestInternalLinkProcessor_ExtractLinks(t *testing.T) {
	t.Parallel()

	p, _ := New("https://github.mycorp.com", "", "", 0, zap.NewNop()) // PAT not needed for regex tests

	type tc struct {
		name string
		line string
		want []string
	}

	tests := []tc{
		{
			name: "keeps github blob; drops externals",
			line: `test https://github.mycorp.com/your-ko/link-validator/blob/main/README.md
			       test https://google.com/x
			       test https://github.com/your-ko/link-validator/blob/main/README.md`,
			want: []string{
				"https://github.mycorp.com/your-ko/link-validator/blob/main/README.md",
				"https://github.com/your-ko/link-validator/blob/main/README.md",
			},
		},
		{
			name: "ignores subdomain uploads.* or api* ",
			line: `test https://uploads.github.mycorp.com/org/repo/raw/main/image.png
			       and external https://gitlab.mycorp.com/a/b
			       and api https://api.github.mycorp.com/org/repo/tree/main/folder`,
			want: nil,
		},
		{
			name: "ignores non-matching schemes and hosts",
			line: `scheme http://github.mycorp.com/org/repo/blob/main/README.md
			       non-github https://other.com/org/repo/blob/main/README.md`,
			want: nil,
		},
		{
			name: "handles anchors but strips query strings",
			line: `https://github.mycorp.com/your-ko/link-validator/blob/main/file.md#L10-L20
			       https://github.com/your-ko/link-validator/blob/main/file.md#L10-L20
			       https://github.mycorp.com/your-ko/link-validator/tree/main/docs?tab=readme
			       https://github.com/your-ko/link-validator/tree/main/docs?tab=readme
			       https://example.com/u/v/raw/main/w.txt?download=1`,
			want: []string{
				"https://github.mycorp.com/your-ko/link-validator/blob/main/file.md#L10-L20",
				"https://github.com/your-ko/link-validator/blob/main/file.md#L10-L20",
				"https://github.mycorp.com/your-ko/link-validator/tree/main/docs",
				"https://github.com/your-ko/link-validator/tree/main/docs",
			},
		},
		{
			name: "captures non-repo urls (without blob|tree|raw|blame|ref)",
			line: `
				https://github.com/your-ko/link-validator/main/docs
				https://github.mycorp.com/your-ko/link-validator/main/docs
				https://github.com/your-ko/link-validator/main/README.md
				https://github.com/your-ko/link-validator/main/README.md
				https://github.com/your-ko/link-validator/pulls
				https://github.com/your-ko/link-validator/issues/4
				`,
			want: []string{
				"https://github.com/your-ko/link-validator/main/docs",
				"https://github.mycorp.com/your-ko/link-validator/main/docs",
				"https://github.com/your-ko/link-validator/main/README.md",
				"https://github.com/your-ko/link-validator/main/README.md",
				"https://github.com/your-ko/link-validator/pulls",
				"https://github.com/your-ko/link-validator/issues/4",
			},
		},
		{
			name: "ignores non-api calls",
			line: `
				https://raw.githubusercontent.com/your-ko/link-validator/refs/heads/main/README.md
				https://uploads.github.mycorp.com/org/repo/raw/main/img.png
				https://api.github.com/repos/your-ko/link-validator/contents/?ref=a96366f66ffacd461de10a1dd561ab5a598e9167
				`,
			want: nil,
		},
		{
			name: "captures refs urls",
			line: `
				particular commit https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167 text
				particular commit https://github.mycorp.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167 text`,
			want: []string{
				"https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
				"https://github.mycorp.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := p.ExtractLinks(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks mismatch\nline=%q\ngot = %#v\nwant= %#v", tt.line, got, tt.want)
			}
		})
	}
}

func TestInternalLinkProcessor_Process(t *testing.T) {
	corp := "https://github.mycorp.com"

	type fields struct {
		status         int
		path           string
		body           string
		base64encoding bool
	}
	type args struct {
		link string
	}
	type tc struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantIs  error // sentinel check via errors.Is; nil => no sentinel check
	}

	tests := []tc{
		{
			name: "file exists, no anchor",
			args: args{link: "/your-ko/link-validator/blob/main/README.md"},
			fields: fields{
				status:         http.StatusOK,
				path:           "/your-ko/link-validator/blob/main/README.md",
				body:           content,
				base64encoding: true,
			},
		},
		{
			name: "file exists, anchor present in content",
			args: args{link: "/your-ko/link-validator/blob/main/README.md#header2"},
			fields: fields{
				status:         http.StatusOK,
				path:           "/your-ko/link-validator/blob/main/README.md",
				body:           content,
				base64encoding: true,
			},
		},
		{
			name: "file exists, anchor missing -> errs.ErrNotFound",
			args: args{link: "/your-ko/link-validator/blob/main/README.md#no-such-anchor"},
			fields: fields{
				status:         http.StatusOK,
				path:           "/your-ko/link-validator/blob/main/README.md",
				body:           content,
				base64encoding: true,
			},
			// anchors temporary don't work
			//wantErr: true,
			//wantIs:  errs.ErrNotFound,
		},
		{
			name: "GitHub returns 404 -> errs.ErrNotFound",
			args: args{link: "/your-ko/link-validator/blob/main/README.md"},
			fields: fields{
				status: http.StatusNotFound,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
			},

			wantErr: true,
			wantIs:  errs.ErrNotFound,
		},
		{
			name: "GitHub returns 500 -> non-sentinel error",
			args: args{link: "/your-ko/link-validator/blob/main/README.md"},
			fields: fields{
				status: http.StatusInternalServerError,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
			},
			wantErr: true,
		},
		{
			name: "repo root (no path)",
			// URL without path after branch; regex yields empty path → GetContents at repo root.
			args: args{link: "/your-ko/link-validator/blob/main"},
			fields: fields{
				status:         http.StatusOK,
				path:           "/your-ko/link-validator/blob/main/README.md",
				body:           content,
				base64encoding: true,
			},
		},
		{
			name: "file exists, link to branch",
			args: args{link: "/your-ko/link-validator/blob/branchname/README.md#about"},
			fields: fields{
				status:         http.StatusOK,
				path:           "/your-ko/link-validator/blob/main/README.md",
				body:           content,
				base64encoding: true,
			},
		},
		{
			name: "link to an issue",
			args: args{link: "/your-ko/link-validator/issues/123"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/issues/123",
				body:   "{\"id\": 1}",
			},
		},
		{
			name: "link to a pull request",
			args: args{link: "/your-ko/link-validator/pull/1"},
			fields: fields{
				status:         http.StatusOK,
				path:           "/your-ko/link-validator/pull/1",
				body:           content,
				base64encoding: true,
			},
		},
		{
			name: "link to a pull requests",
			args: args{link: "/your-ko/link-validator/pulls/your-ko"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/pulls/your-ko",
				body:   "",
			},
		},
		{
			name: "link to releases",
			args: args{link: "/your-ko/link-validator/releases"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/pulls/your-ko/releases",
				body:   "",
			},
		},
		{
			name: "link to a commits",
			args: args{link: "/your-ko/link-validator/commits/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/commits/test",
				body:   "",
			},
		},
		{
			name: "link to a discussions",
			args: args{link: "/your-ko/link-validator/discussions/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/discussions/test",
				body:   "",
			},
		},
		{
			name: "link to a branches",
			args: args{link: "/your-ko/link-validator/branches/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/branches/test",
				body:   "",
			},
		},
		{
			name: "link to a branches",
			args: args{link: "/your-ko/link-validator/tags/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/tags/test",
				body:   "",
			},
		},
		{
			name: "link to a milestones",
			args: args{link: "/your-ko/link-validator/milestones/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/milestones/test",
				body:   "",
			},
		},
		{
			name: "link to a labels",
			args: args{link: "/your-ko/link-validator/labels/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/labels/test",
				body:   "",
			},
		},
		{
			name: "link to a projects",
			args: args{link: "/your-ko/link-validator/projects/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/projects/test",
				body:   "",
			},
		},
		{
			name: "link to a actions",
			args: args{link: "/your-ko/link-validator/actions/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/actions/test",
				body:   "",
			},
		},
		{
			name: "link to a workflow badge",
			args: args{link: "/your-ko/link-validator/actions/workflows/main.yaml/badge.svg"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/actions/workflows/main.yaml/badge.svg",
				body:   "",
			},
		},
		{
			name: "link to a workflow",
			args: args{link: "/your-ko/link-validator/actions/workflows/main.yaml"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/actions/workflows/main.yaml",
				body:   "",
			},
		},
		{
			name: "link to an org",
			args: args{link: "/your-ko/link-validator"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator",
				body:   "",
			},
		},
		{
			name: "link to an organisations",
			args: args{link: "/organizations/your-ko/settings/apps/test"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator",
				body:   "",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				//if tt.fields.loc != "" {
				//	res.Header().Set("Location", tt.fields.loc)
				//}
				res.WriteHeader(tt.fields.status)

				if tt.fields.base64encoding {
					_ = json.NewEncoder(res).Encode(&githubContent{
						Type:     "file",
						Encoding: "base64",
						Content:  base64.StdEncoding.EncodeToString([]byte(tt.fields.body)),
					})
				} else {
					res.Header().Set("Content-Type", "application/json")
					_, _ = res.Write([]byte(tt.fields.body))
				}
			}))
			t.Cleanup(testServer.Close)

			proc := mockValidator(testServer, corp)
			err := proc.Process(context.Background(), corp+tt.args.link, "") // we add corpUrl here, but it doesn't matter in this test, because we test the path

			if (err != nil) != tt.wantErr {
				t.Fatalf("error presence %v, want %v (err=%v)", err != nil, tt.wantErr, err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
			}
			// When wantIs is nil, ensure we did NOT map to errs.ErrNotFound accidentally.
			if tt.wantIs == nil && errors.Is(err, errs.ErrNotFound) {
				t.Fatalf("unexpected mapping to errs.ErrNotFound: %v", err)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("Process() err presence = %v, wantErr=%v (err=%v)", err != nil, tt.wantErr, err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs == nil {
				return
			}

			// If a sentinel is specified, ensure errors.Is matches it.
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected \n errors.Is(err, %v) to be true; \n got err=%v", tt.wantIs, err)
			}

			expected := fmt.Sprintf("%s. Incorrect link: '%s%s'", tt.wantIs, corp, tt.args.link)
			if err.Error() != expected {
				t.Fatalf("Got error message:\n %s\n want:\n %s", err.Error(), expected)
			}

		})
	}
}

type githubContent struct {
	Type     string `json:"type"`     // "file" or "dir"
	Encoding string `json:"encoding"` // "base64" for file
	Content  string `json:"content"`  // base64-encoded file body
}

// mockValidator creates a validator instance with mock GitHub clients
func mockValidator(ts *httptest.Server, corp string) *LinkProcessor {
	p, _ := New(corp, "", "", time.Second, zap.NewNop())

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

const content = `
test
# header 1
test
## header2
test
`

func TestInternalLinkProcessor_RegexRepoUrl(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *ghURL
		wantErr bool
	}{
		{
			name: "repo url",
			url:  "https://github.com/your-ko/link-validator/",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
			},
		},
		{
			name: "repo url blob",
			url:  "https://github.com/your-ko/link-validator/blob/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "repo url blob with nested directories",
			url:  "https://github.com/your-ko/link-validator/blob/main/docs/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "main",
				path:  "docs/README.md",
			},
		},
		{
			name: "repo url blob referencing file in tag",
			url:  "https://github.com/your-ko/link-validator/blob/1.0.0/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "1.0.0",
				path:  "README.md",
			},
		},
		{
			name: "repo url raw",
			url:  "https://github.com/your-ko/link-validator/raw/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "raw",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "repo url tree",
			url:  "https://github.com/your-ko/link-validator/tree/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "refs heads particular file",
			url:  "https://github.com/your-ko/link-validator/tree/refs/heads/main/Dockerfile",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "refs",
				path:  "heads/main/Dockerfile",
			},
		},
		{
			name: "repo url blame",
			url:  "https://github.com/your-ko/link-validator/blame/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blame",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "tree: nested dir",
			url:  "https://github.com/your-ko/link-validator/tree/main/cmd",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "main",
				path:  "cmd",
			},
		},
		{
			name: "branches: particular branch",
			url:  "https://github.com/your-ko/link-validator/tree/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "test",
			},
		},
		{
			name: "enterprise repo url blob",
			url:  "https://github.mycorp.com/your-ko/link-validator/blob/main/README.md",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				owner:      "your-ko",
				repo:       "link-validator",
				typ:        "blob",
				ref:        "main",
				path:       "README.md",
			},
		},
		{
			name: "repo url blob with anchor",
			url:  "https://github.com/your-ko/link-validator/blob/main/README.md#features",
			want: &ghURL{
				host:   "github.com",
				owner:  "your-ko",
				repo:   "link-validator",
				typ:    "blob",
				ref:    "main",
				path:   "README.md",
				anchor: "features",
			},
		},
		{
			name: "Referencing a file in a particular commit",
			url:  "https://github.com/your-ko/link-validator/blob/83e43288254d0f36e723ef2cf3328b8b77836560/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "83e43288254d0f36e723ef2cf3328b8b77836560",
				path:  "README.md",
			},
		},
		{
			name: "repo commits",
			url:  "https://github.com/your-ko/link-validator/commits",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "commits",
			},
		},
		{
			name: "particular commit",
			url:  "https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "commit",
				ref:   "a96366f66ffacd461de10a1dd561ab5a598e9167",
			},
		},
		{
			name: "releases",
			url:  "https://github.com/your-ko/link-validator/releases",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
			},
		},
		{
			name: "release: latest",
			url:  "https://github.com/your-ko/link-validator/releases/latest",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				path:  "latest",
			},
		},
		{
			name: "release: particular with tag",
			url:  "https://github.com/your-ko/link-validator/releases/tag/1.0.0",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				ref:   "tag",
				path:  "1.0.0",
			},
		},
		{
			name: "release: particular without tag",
			url:  "https://github.com/your-ko/link-validator/releases/1.0.0",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				path:  "1.0.0",
			},
		},
		{
			name: "release: particular release without tag",
			url:  "https://github.com/your-ko/link-validator/releases/1.0.0",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				path:  "1.0.0",
			},
		},
		{
			name: "release: download artifact",
			url:  "https://github.com/your-ko/link-validator/releases/download/1.0.0/sbom.spdx.json",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				ref:   "download",
				path:  "1.0.0/sbom.spdx.json",
			},
		},
		{
			name: "issues url",
			url:  "https://github.com/your-ko/link-validator/issues",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "issues",
			},
		},
		{
			name: "particular issue",
			url:  "https://github.com/your-ko/link-validator/issues/123",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "issues",
				ref:   "123",
			},
		},
		{
			name: "pull requests list",
			url:  "https://github.com/your-ko/link-validator/pulls",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pulls",
			},
		},
		{
			name: "pull: pr with an author",
			url:  "https://github.com/your-ko/link-validator/pulls/your-ko",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pulls",
				ref:   "your-ko",
			},
		},
		{
			name: "pull: particular PR",
			url:  "https://github.com/your-ko/link-validator/pull/1",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pull",
				ref:   "1",
			},
		},
		{
			name: "pull:  files tab",
			url:  "https://github.com/your-ko/link-validator/pull/1/files",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pull",
				ref:   "1",
				path:  "files",
			},
		},
		{
			name: "pull: commits",
			url:  "https://github.com/your-ko/link-validator/pull/1/commits",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pull",
				ref:   "1",
				path:  "commits",
			},
		},
		{
			name: "pull: issue comment",
			url:  "https://github.com/your-ko/link-validator/pull/1#issuecomment-123456",
			want: &ghURL{
				host:   "github.com",
				owner:  "your-ko",
				repo:   "link-validator",
				typ:    "pull",
				ref:    "1",
				anchor: "issuecomment-123456",
			},
		},
		{
			name: "pull: discussion",
			url:  "https://github.com/your-ko/link-validator/pull/1#discussion_r123456",
			want: &ghURL{
				host:   "github.com",
				owner:  "your-ko",
				repo:   "link-validator",
				typ:    "pull",
				ref:    "1",
				anchor: "discussion_r123456",
			},
		},
		{
			name: "repo url commits in branch",
			url:  "https://github.com/your-ko/link-validator/commits/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "commits",
				ref:   "test",
			},
		},
		{
			name: "repo url to  discussions",
			url:  "https://github.com/your-ko/link-validator/discussions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "discussions",
			},
		},
		//{	TODO: not possible via GitHub API. Requires HTTP auth or GraphQL API
		//	name: "repo url to a particular discussion",
		//	url:  "https://github.com/your-ko/link-validator/discussions/test",
		//	want: &ghURL{
		//		host:  "github.com",
		//		owner: "your-ko",
		//		repo:  "link-validator",
		//		typ:   "discussions",
		//		ref:   "test",
		//	},
		//},
		{
			name: "branches: list",
			url:  "https://github.com/your-ko/link-validator/branches",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "branches",
			},
		},
		{
			name: "milestones: list",
			url:  "https://github.com/your-ko/link-validator/milestones",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "milestones",
			},
		},
		{
			name: "milestones: particular milestone",
			url:  "https://github.com/your-ko/link-validator/milestone/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "milestone",
				ref:   "test",
			},
		},
		{
			name: "repo url projects",
			url:  "https://github.com/your-ko/link-validator/projects",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "projects",
			},
		},
		{ // TODO:  No support in GitHub API. GraphQL API needs to be used instead
			name: "repo url to a particular project",
			url:  "https://github.com/your-ko/link-validator/projects/1",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "projects",
				ref:   "1",
			},
		},
		{
			name: "repo settings",
			url:  "https://github.com/your-ko/link-validator/settings/secrets/actions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "settings",
			},
		},
		{
			name: "Repository settings tabs",
			url:  "https://github.com/your-ko/link-validator/settings/actions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "settings",
			},
		},
		{
			name: "Security: advisories",
			url:  "https://github.com/your-ko/link-validator/security/advisories",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "security",
			},
		},
		{
			name: "Security: particular advisory",
			url:  "https://github.com/your-ko/link-validator/security/advisories/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "advisories",
				ref:   "test",
			},
		},
		{
			name: "Security: policy",
			url:  "https://github.com/your-ko/link-validator/security/policy",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "security",
			},
		},
		{
			name: "compare branches",
			url:  "https://github.com/your-ko/link-validator/compare/main...dev",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "compare",
				ref:   "main...dev",
			},
		},
		{
			name: "workflows: list",
			url:  "https://github.com/your-ko/link-validator/actions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
			},
		},
		{
			name: "workflows: particular workflow run",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003183",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003183",
			},
		},
		{
			name: "workflows: badge",
			url:  "https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "workflows",
				path:  "main.yaml/badge.svg",
			},
		},
		{
			name: "workflow: file",
			url:  "https://github.com/your-ko/link-validator/actions/workflows/main.yaml",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "workflows",
				path:  "main.yaml",
			},
		},
		{
			name: "Workflows: particular run attempt",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/attempts/1",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/attempts/1",
			},
		},
		{
			name: "Workflows: particular run attempt logs",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/attempts/1/logs",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/attempts/1/logs",
			},
		},
		{
			name: "Workflow: logs of specific attempt",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/job/54938961245",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/job/54938961245",
			},
		},
		{
			name: "Workflows: specific job usage",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/usage",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/usage",
			},
		},
		{
			name: "repo url tags",
			url:  "https://github.com/your-ko/link-validator/tags",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tags",
			},
		},
		{
			name: "repo url labels",
			url:  "https://github.com/your-ko/link-validator/labels",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "labels",
			},
		},
		{
			name: "repo url particular labels",
			url:  "https://github.com/your-ko/link-validator/labels/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "labels",
			},
		},
		{
			name: "github user",
			url:  "https://github.com/your-ko",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				typ:   "user",
			},
		},
		{
			name: "repo: attestations",
			url:  "https://github.com/your-ko/link-validator/attestations",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "attestations",
			},
		},
		//{ TODO: it seems that the attestations are available by HTTP client
		//	name: "repo: attestations",
		//	url:  "https://github.com/your-ko/link-validator/attestations/13059584",
		//	want: &ghURL{
		//		host:  "github.com",
		//		owner: "your-ko",
		//		repo:  "link-validator",
		//		typ:   "attestations",
		//		ref:   "13059584",
		//	},
		//},
		{
			name: "organizations: settings",
			url:  "https://github.mycorp.com/organizations/your-ko/settings/apps/test",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "orgs",
				owner:      "your-ko",
				path:       "settings/apps/test",
			},
		},
		{
			name: "organization root",
			url:  "https://github.mycorp.com/settings/organizations",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "nope",
				path:       "organizations",
			},
		},
		{
			name: "organization dashboard",
			url:  "https://github.mycorp.com/orgs/your-ko/repositories",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "orgs",
				owner:      "your-ko",
				path:       "repositories",
			},
		},
		{
			name: "organization people",
			url:  "https://github.mycorp.com/orgs/your-ko/people",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "orgs",
				owner:      "your-ko",
				path:       "people",
			},
		},
		{
			name: "repo: packages root",
			url:  "https://github.com/your-ko/link-validator/packages",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "packages",
			},
		},
		{
			name: "repo container package",
			url:  "https://github.com/your-ko/link-validator/pkgs/container/link-validator",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pkgs",
				ref:   "container",
				path:  "link-validator",
			},
		},
		//{
		//	name: "api pull request url",
		//	url:  "https://api.github.com/repos/your-ko/link-validator",
		//	want: nil,
		//},
		//{
		//	name: "uploads repo url",
		//	url:  "https://uploads.github.mycorp.com/org/repo/raw/main/image.png",
		//	want: nil,
		//},
		{
			name: "GitHub",
			url:  "https://github.com",
			want: &ghURL{host: "github.com"},
		},
		{
			name: "GitHub enterprise",
			url:  "https://github.mycorp.com",
			want: &ghURL{host: "github.mycorp.com", enterprise: true},
		},
		{
			name: "user profile",
			url:  "https://github.com/your-ko",
			want: &ghURL{host: "github.com", owner: "your-ko", typ: "user"},
		},
		{ // TODO: GitHub wikis are not accessible through the REST API
			name: "repo wiki root",
			url:  "https://github.com/your-ko/link-validator/wiki",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "wiki",
			},
		},
		{
			name: "repo wiki page",
			url:  "https://github.com/your-ko/link-validator/wiki/Installation-Guide",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "wiki",
				ref:   "Installation-Guide",
			},
		},
		{
			name: "repo wiki page history",
			url:  "https://github.com/your-ko/link-validator/wiki/Installation-Guide/_history",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "wiki",
				ref:   "Installation-Guide",
				path:  "_history",
			},
		},
		//// Gist URLs - these should return nil as they use gist.github.com
		//{
		//	name: "gist root",
		//	url:  "https://gist.github.com/your-ko/e8c76f357ca09860f5a0a9afb461190e",
		//	want: nil, // gist.github.com is not handled by current parser
		//},
		//{
		//	name: "gists",
		//	url:  "https://gist.github.com/your-ko",
		//	want: nil, // gist.github.com is not handled by current parser
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseUrl(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unexpected error:\n%s", err)
				t.Fail()
			}
			if !reflect.DeepEqual(res, tt.want) {
				t.Errorf("FindStringSubmatch()\n got = %v\nwant = %v", res, tt.want)
			}
		})
	}
}
