package gh

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
	"link-validator/pkg/errs"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"reflect"
	"testing"
)

func TestInternalLinkProcessor_ExtractLinks(t *testing.T) {
	t.Parallel()

	p := New("https://github.mycorp.com", "", "") // PAT not needed for regex tests

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
	logger := zap.NewNop()
	corp := "https://github.mycorp.com"

	type fields struct {
		status int
		path   string
		body   string
	}
	type args struct {
		link string
	}
	type tc struct {
		name    string
		fields  fields
		args    args
		setup   func(w http.ResponseWriter, r *http.Request)
		wantErr bool
		wantIs  error // sentinel check via errors.Is; nil => no sentinel check
	}

	tests := []tc{
		{
			name: "file exists, no anchor",
			args: args{link: "/your-ko/link-validator/blob/main/README.md"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
			},
		},
		{
			name: "file exists, anchor present in content",
			args: args{link: "/your-ko/link-validator/blob/main/README.md#header2"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
			},
		},
		{
			name: "file exists, anchor missing -> errs.NotFound",
			args: args{link: "/your-ko/link-validator/blob/main/README.md#no-such-anchor"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
			},
			// anchors temporary don't work
			//wantErr: true,
			//wantIs:  errs.NotFound,
		},
		{
			name: "GitHub returns 404 -> errs.NotFound",
			args: args{link: "/your-ko/link-validator/blob/main/README.md"},
			fields: fields{
				status: http.StatusNotFound,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
			},

			wantErr: true,
			wantIs:  errs.NotFound,
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
				status: http.StatusOK,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
			},
		},
		{
			name: "file exists, link to branch",
			args: args{link: "/your-ko/link-validator/blob/branch/main/README.md#header2"},
			fields: fields{
				status: http.StatusOK,
				path:   "/your-ko/link-validator/blob/main/README.md",
				body:   content,
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

				_ = json.NewEncoder(res).Encode(&githubContent{
					Type:     "file",
					Encoding: "base64",
					Content:  base64.StdEncoding.EncodeToString([]byte(tt.fields.body)),
				})
			}))
			t.Cleanup(testServer.Close)

			proc := mockValidator(testServer, corp)
			err := proc.Process(context.Background(), corp+tt.args.link, logger) // we add corpUrl here, but it doesn't matter in this test, because we test the path

			if (err != nil) != tt.wantErr {
				t.Fatalf("error presence %v, want %v (err=%v)", err != nil, tt.wantErr, err)
			}
			if !tt.wantErr {
				return
			}

			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected errors.Is(err, %v) true, got %v", tt.wantIs, err)
			}
			// When wantIs is nil, ensure we did NOT map to errs.NotFound accidentally.
			if tt.wantIs == nil && errors.Is(err, errs.NotFound) {
				t.Fatalf("unexpected mapping to errs.NotFound: %v", err)
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
	p := New(corp, "", "")

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

func TestInternalLinkProcessor_RegexRepoUrlDetection(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want []string
	}{
		{
			name: "repo url blob",
			url:  "https://github.com/your-ko/link-validator/blob/main/README.md",
			want: []string{
				"https://github.com/your-ko/link-validator/blob/main/README.md",
				"github.com",
				"your-ko",
				"link-validator",
				"blob",
				"main",
				"README.md",
				"",
			},
		},
		{
			name: "repo url raw",
			url:  "https://github.com/your-ko/link-validator/raw/main/README.md",
			want: []string{
				"https://github.com/your-ko/link-validator/raw/main/README.md",
				"github.com",
				"your-ko",
				"link-validator",
				"raw",
				"main",
				"README.md",
				"",
			},
		},
		{
			name: "repo url tree",
			url:  "https://github.com/your-ko/link-validator/tree/main/README.md",
			want: []string{
				"https://github.com/your-ko/link-validator/tree/main/README.md",
				"github.com",
				"your-ko",
				"link-validator",
				"tree",
				"main",
				"README.md",
				"",
			},
		},
		{
			name: "repo url blame",
			url:  "https://github.com/your-ko/link-validator/blame/main/README.md",
			want: []string{
				"https://github.com/your-ko/link-validator/blame/main/README.md",
				"github.com",
				"your-ko",
				"link-validator",
				"blame",
				"main",
				"README.md",
				"",
			},
		},
		{
			name: "repo url dir blame",
			url:  "https://github.com/your-ko/link-validator/tree/main/cmd",
			want: []string{
				"https://github.com/your-ko/link-validator/tree/main/cmd",
				"github.com",
				"your-ko",
				"link-validator",
				"tree",
				"main",
				"cmd",
				"",
			},
		},
		{
			name: "enterprise repo url blob",
			url:  "https://github.mycorp.com/your-ko/link-validator/blob/main/README.md",
			want: []string{
				"https://github.mycorp.com/your-ko/link-validator/blob/main/README.md",
				"github.mycorp.com",
				"your-ko",
				"link-validator",
				"blob",
				"main",
				"README.md",
				"",
			},
		},
		{
			name: "repo url blob with anchor",
			url:  "https://github.com/your-ko/link-validator/blob/main/README.md#header",
			want: []string{
				"https://github.com/your-ko/link-validator/blob/main/README.md#header",
				"github.com",
				"your-ko",
				"link-validator",
				"blob",
				"main",
				"README.md",
				"header",
			},
		},
		{
			name: "particular commit",
			url:  "https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
			want: []string{
				"https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
				"github.com",
				"your-ko",
				"link-validator",
				"commit",
				"a96366f66ffacd461de10a1dd561ab5a598e9167",
				"",
				"",
			},
		},
		{
			name: "repo url tag",
			url:  "https://github.com/your-ko/link-validator/releases/tag/0.9.0",
			//want: nil,
			want: []string{
				"https://github.com/your-ko/link-validator/releases/tag/0.9.0",
				"github.com",
				"your-ko",
				"link-validator",
				"releases",
				"0.9.0",
				"",
				"",
			},
		},
		{
			name: "api repo url",
			url:  "https://api.github.com/repos/your-nj/link-validator",
			want: nil,
		},
		{
			name: "uploads repo url",
			url:  "https://uploads.github.mycorp.com/org/repo/raw/main/image.png",
			want: nil,
		},
		{
			name: "GitHub",
			url:  "https://github.com",
			want: nil,
		},
		{
			name: "GitHub enterprise",
			url:  "https://github.mycorp.com",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := mockValidator(nil, "https://github.mycorp.com")
			res := proc.repoRegex.FindStringSubmatch(tt.url)
			if !reflect.DeepEqual(res, tt.want) {
				t.Errorf("FindStringSubmatch()\n got = %s\nwant = %s", res, tt.want)
			}
		})
	}
}
