package intern

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

	p := New("https://github.mycorp.com", "") // PAT not needed for regex tests

	type tc struct {
		name string
		line string
		want []string
	}

	tests := []tc{
		{
			name: "keeps internal blob/tree/raw; drops externals",
			line: `see https://github.mycorp.com/org/repo/blob/main/README.md
			       and https://google.com/x
			       then https://github.mycorp.com/org/repo/tree/main/dir
			       and https://example.com/y
			       and https://github.mycorp.com/org/repo/raw/main/file.txt`,
			want: []string{
				"https://github.mycorp.com/org/repo/blob/main/README.md",
				"https://github.mycorp.com/org/repo/tree/main/dir",
				"https://github.mycorp.com/org/repo/raw/main/file.txt",
			},
		},
		{
			name: "includes subdomain uploads.* as internal",
			line: `assets at https://uploads.github.mycorp.com/org/repo/raw/main/image.png
			       and external https://gitlab.mycorp.com/a/b
			       and internal https://github.mycorp.com/acme/proj/blob/main/notes.md`,
			want: []string{
				"https://uploads.github.mycorp.com/org/repo/raw/main/image.png",
				"https://github.mycorp.com/acme/proj/blob/main/notes.md",
			},
		},
		{
			name: "ignores non-matching schemes and hosts",
			line: `http://github.mycorp.com/org/repo/blob/main/README.md
			       https://other.com/org/repo/blob/main/README.md
			       https://api.github.mycorp.com/org/repo/tree/main/folder`,
			want: []string{
				"https://api.github.mycorp.com/org/repo/tree/main/folder",
			},
		},
		{
			name: "handles anchors and query strings",
			line: `https://github.mycorp.com/team/proj/blob/main/file.md#L10-L20
			       https://github.mycorp.com/team/proj/tree/main/docs?tab=readme
			       https://example.com/u/v/raw/main/w.txt?download=1`,
			want: []string{
				"https://github.mycorp.com/team/proj/blob/main/file.md#L10-L20",
				"https://github.mycorp.com/team/proj/tree/main/docs?tab=readme",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := p.ExtractLinks(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks mismatch\nbase=%q\nline=%q\ngot = %#v\nwant= %#v",
					p.corpGitHubUrl, tt.line, got, tt.want)
			}
		})
	}
}

func TestInternalLinkProcessor_Process(t *testing.T) {
	logger := zap.NewNop()
	corp := "https://github.example.com" // enterprise host used for link parsing

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
			args: args{link: "/acme/proj/blob/main/README.md"},
			fields: fields{
				status: http.StatusOK,
				path:   "/acme/proj/blob/main/README.md",
				body:   content,
			},
		},
		{
			name: "file exists, anchor present in content",
			args: args{link: "/acme/proj/blob/main/README.md#header2"},
			fields: fields{
				status: http.StatusOK,
				path:   "/acme/proj/blob/main/README.md",
				body:   content,
			},
		},
		{
			name: "file exists, anchor missing -> errs.NotFound",
			args: args{link: "/acme/proj/blob/main/README.md#no-such-anchor"},
			fields: fields{
				status: http.StatusOK,
				path:   "/acme/proj/blob/main/README.md",
				body:   content,
			},
			wantErr: true,
			wantIs:  errs.NotFound,
		},
		{
			name: "GitHub returns 404 -> errs.NotFound",
			args: args{link: "/acme/proj/blob/main/README.md"},
			fields: fields{
				status: http.StatusNotFound,
				path:   "/acme/proj/blob/main/README.md",
				body:   content,
			},

			wantErr: true,
			wantIs:  errs.NotFound,
		},
		{
			name: "GitHub returns 500 -> non-sentinel error",
			args: args{link: "/acme/proj/blob/main/README.md"},
			fields: fields{
				status: http.StatusInternalServerError,
				path:   "/acme/proj/blob/main/README.md",
				body:   content,
			},
			wantErr: true,
		},
		{
			name: "repo root (no path)",
			// URL without path after branch; regex yields empty path → GetContents at repo root.
			args: args{link: "/acme/proj/blob/main"},
			fields: fields{
				status: http.StatusOK,
				path:   "/acme/proj/blob/main/README.md",
				body:   content,
			},
		},
		{
			name: "file exists, link to branch",
			args: args{link: "/acme/proj/blob/branch/main/README.md#header2"},
			fields: fields{
				status: http.StatusOK,
				path:   "/acme/proj/blob/main/README.md",
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
			err := proc.Process(context.Background(), corp+tt.args.link, logger)

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

// mockValidator creates a validator instance with our enterprise host (used by regex),
// then replace its client with one that points to the test server.
func mockValidator(ts *httptest.Server, corp string) *InternalLinkProcessor {
	p := New(corp, "")
	c := github.NewClient(ts.Client())
	base, _ := neturl.Parse(ts.URL + "/")
	c.BaseURL = base
	c.UploadURL = base
	p.client = c
	return p
}

const content = `
test
# header 1
test
## header2
test
`
