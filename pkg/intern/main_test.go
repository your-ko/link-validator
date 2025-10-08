package intern

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/google/go-github/v74/github"
	"go.uber.org/zap"
	"link-validator/pkg/errs"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"reflect"
	"strings"
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

	type tc struct {
		name    string
		link    string
		setup   func(w http.ResponseWriter, r *http.Request)
		wantErr bool
		wantIs  error // sentinel check via errors.Is; nil => no sentinel check
	}

	tests := []tc{
		{
			name: "file exists, no anchor -> nil",
			link: corp + "/acme/proj/blob/main/README.md",
			setup: func(w http.ResponseWriter, r *http.Request) {
				// Expect GET /repos/acme/proj/contents/README.md?ref=main
				if r.Method != http.MethodGet {
					t.Fatalf("unexpected method: %s", r.Method)
				}
				_ = json.NewEncoder(w).Encode(&ghContent{
					Type:     "file",
					Encoding: "base64",
					Content:  base64.StdEncoding.EncodeToString([]byte("hello\nworld\n")),
				})
			},
		},
		{
			name: "file exists, anchor present in content -> nil",
			link: corp + "/acme/proj/blob/main/README.md#anchor-123",
			setup: func(w http.ResponseWriter, r *http.Request) {
				body := "intro\n#anchor-123\nrest\n"
				_ = json.NewEncoder(w).Encode(&ghContent{
					Type:     "file",
					Encoding: "base64",
					Content:  base64.StdEncoding.EncodeToString([]byte(body)),
				})
			},
		},
		{
			name: "file exists, anchor missing -> errs.NotFound",
			link: corp + "/acme/proj/blob/main/README.md#no-such-anchor",
			setup: func(w http.ResponseWriter, r *http.Request) {
				body := "intro\n#some-other-anchor\n"
				_ = json.NewEncoder(w).Encode(&ghContent{
					Type:     "file",
					Encoding: "base64",
					Content:  base64.StdEncoding.EncodeToString([]byte(body)),
				})
			},
			wantErr: true,
			wantIs:  errs.NotFound,
		},
		{
			name: "GitHub returns 404 -> errs.NotFound",
			link: corp + "/acme/proj/blob/main/README.md",
			setup: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			},
			wantErr: true,
			wantIs:  errs.NotFound,
		},
		{
			name: "GitHub returns 500 -> non-sentinel error",
			link: corp + "/acme/proj/blob/main/README.md",
			setup: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"boom"}`))
			},
			wantErr: true,
			// wantIs nil => assert not mapped to errs.NotFound below
		},
		{
			name: "repo root (no path) -> nil",
			// URL without path after branch; regex yields empty path → GetContents at repo root.
			link: corp + "/acme/proj/blob/main",
			setup: func(w http.ResponseWriter, r *http.Request) {
				// Return a directory listing ([]), fileContent=nil in go-github terms.
				// Any JSON array is fine here.
				_, _ = w.Write([]byte(`[]`))
			},
		},
		{
			name: "invalid URL for this processor -> error",
			link: "https://other.example.com/owner/repo/blob/main/README.md",
			setup: func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("server should not be called for invalid URL")
			},
			wantErr: true,
			// error is a normal fmt.Errorf("invalid or unsupported...") – not a sentinel
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Ensure the expected endpoint shape (best-effort).
				if got, want := r.URL.Path, "/repos/acme/proj/contents/README.md"; strings.Contains(tt.link, "README.md") {
					// For README cases we expect this path; for other cases (root/no path), any /repos/.../contents is OK.
					if !strings.HasPrefix(got, "/repos/acme/proj/contents") {
						t.Fatalf("unexpected API path: %s", got)
					}
				}
				tt.setup(w, r)
			}))
			t.Cleanup(ts.Close)

			proc := newProcPointingTo(ts, corp)
			err := proc.Process(context.Background(), tt.link, logger)

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
		})
	}
}

type ghContent struct {
	Type     string `json:"type"`     // "file" or "dir"
	Encoding string `json:"encoding"` // "base64" for file
	Content  string `json:"content"`  // base64-encoded file body
}

func newProcPointingTo(ts *httptest.Server, corp string) *InternalLinkProcessor {
	// Create a proc with our enterprise host (used by regex),
	// then replace its client with one that points to the test server.
	p := New(corp, "")
	c := github.NewClient(ts.Client())
	base, _ := neturl.Parse(ts.URL + "/")
	c.BaseURL = base
	c.UploadURL = base
	p.client = c
	return p
}
