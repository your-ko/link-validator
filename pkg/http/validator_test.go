package http

import (
	"context"
	"errors"
	"link-validator/pkg/errs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestExternalHttpLinkProcessor_ExtractLinks(t *testing.T) {
	t.Parallel()

	type tc struct {
		name string
		line string
		want []string
	}

	tests := []tc{
		{
			name: "drop github blob; keep externals",
			line: `test https://github.mycorp.com/your-ko/link-validator/blob/main/README.md
			       test https://google.com/x
			       test https://github.com/your-ko/link-validator/blob/main/README.md`,
			want: []string{
				"https://google.com/x",
			},
		},
		{
			name: "Ignores templated urls",
			line: `test https://host.[blah].[blah]/path/path
				   https://google.com
			       test https://host.{blah}.{blah}/path/path
			       test https://host.{blah}.{blah}:1234/path/path
			       test https://host.com/{blah}/{blah}/path/path
			       test https://host.{{blah}}.{{blah}}/path/path`,
			want: []string{"https://google.com"},
		},
		{
			name: "Captures urls separated by new line",
			line: `
				test https://google.com\n\nhttps://google.com\thttps://google.com
				"test https://google.com\n\nhttps://google.com\thttps://google.com""
				`,
			want: []string{
				"https://google.com",
				"https://google.com",
				"https://google.com",
				"https://google.com",
				"https://google.com",
				"https://google.com",
			},
		},
		{
			name: "capture subdomain uploads.* or api* ",
			line: `test https://uploads.github.mycorp.com/org/repo/raw/main/image.png
			       and external https://gitlab.mycorp.com/a/b
			       and api https://api.github.mycorp.com/org/repo/tree/main/folder`,
			want: []string{
				"https://uploads.github.mycorp.com/org/repo/raw/main/image.png",
				"https://gitlab.mycorp.com/a/b",
				"https://api.github.mycorp.com/org/repo/tree/main/folder",
			},
		},
		{
			name: "capture GitHub blog ",
			line: `test https://github.blog/changelog/2025-11-18-github-copilot-cli-new-models-enhanced-code-search-and-better-image-support/
			       and https://github.mycorp.com/your-ko/link-validator/blob/main/README.md`,
			want: []string{
				"https://github.blog/changelog/2025-11-18-github-copilot-cli-new-models-enhanced-code-search-and-better-image-support/",
			},
		},
		{
			name: "ignores non-matching schemes, captures another hosts",
			line: `scheme http://github.mycorp.com/org/repo/blob/main/README.md
			       non-github https://other.com/org/repo/blob/main/README.md`,
			want: []string{
				"https://other.com/org/repo/blob/main/README.md",
			},
		},
		{
			name: "handles anchors and query strings",
			line: `https://github.mycorp.com/your-ko/link-validator/blob/main/file.md#L10-L20
			       https://github.com/your-ko/link-validator/blob/main/file.md#L10-L20
			       https://github.mycorp.com/your-ko/link-validator/tree/main/docs?tab=readme
			       https://github.com/your-ko/link-validator/tree/main/docs?tab=readme
			       https://example.com/u/v/raw/main/w.txt#anchor1
			       https://example.com/u/v/raw/main/w.txt?download=1`,
			want: []string{
				"https://example.com/u/v/raw/main/w.txt#anchor1",
				"https://example.com/u/v/raw/main/w.txt?download=1",
			},
		},
		{
			name: "ignores non-repo urls (without blob|tree|raw|blame|ref)",
			line: `
				https://github.com/your-ko/link-validator
				https://github.mycorp.com/bnl/elasticaas/actions/workflows/master.yaml
				https://github.com/your-ko/link-validator/pulls
				https://github.com/your-ko/link-validator/issues/4
				`,
			want: []string{},
		},
		{
			name: "captures non-api calls",
			line: `
				https://uploads.github.mycorp.com/org/repo/raw/main/img.png
				https://raw.githubusercontent.com/your-ko/link-validator/refs/heads/main/README.md
				https://api.github.com/repos/your-ko/link-validator/contents/?ref=a96366f66ffacd461de10a1dd561ab5a598e9167
				`,
			want: []string{
				"https://uploads.github.mycorp.com/org/repo/raw/main/img.png",
				"https://raw.githubusercontent.com/your-ko/link-validator/refs/heads/main/README.md",
				"https://api.github.com/repos/your-ko/link-validator/contents/?ref=a96366f66ffacd461de10a1dd561ab5a598e9167",
			},
		},
		{
			name: "ignores refs urls",
			line: `
				particular commit https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167 text
				particular commit https://github.mycorp.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167 text`,
			want: []string{},
		},
		{
			name: "ignores urls containing special characters",
			line: `https://[github].[mycorp].[com]`,
			want: []string{},
		},
		{
			name: "excludes non-url characters from the end of URLs",
			line: `
				[test](https://www.test.com/) and 
				[Example](https://example.org/docs) and
				https://example.org/docs. and
				[Example](https://example.org/docs/) in markdown`,
			want: []string{
				"https://www.test.com/",
				"https://example.org/docs",
				"https://example.org/docs",
				"https://example.org/docs/",
			},
		},
		{
			name: "ignores app datadog urls",
			line: `test
				https://app.datadoghq.com/metric/explorer?fromUser=false,
				https://app.datadoghq.com/monitors/manage,
				https://app.datadoghq.com/monitors/1234567890,
				https://app.datadoghq.com/on-call/teams,
				https://app.datadoghq.com/dashboard/somepath/somedashboard
				https://github.com/DataDog/datadog-api-client-go/,
				https://docs.datadoghq.com/,
				https://google.com,
				test`,
			want: []string{
				"https://docs.datadoghq.com/",
				"https://google.com",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := New(10, nil)
			got := proc.ExtractLinks(tt.line)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks() mismatch\nline=%q\ngot = %#v\nwant= %#v", tt.line, got, tt.want)
			}
		})
	}
}

func TestHttpLinkProcessor_Process(t *testing.T) {
	t.Parallel()
	type fields struct {
		status int
		body   string
		sleep  time.Duration // optional server delay
		loc    string        // optional redirect Location
	}
	type args struct {
		url string
	}
	tests := []struct {
		name            string
		fields          fields
		args            args
		wantErr         bool
		wantIs          error
		expectNoRequest bool // true => server handler must not be hit (excluded host short-circuit)
		timeoutClient   bool // true => override client with short timeout; expect non-sentinel error
	}{
		{
			name:    "200 with body",
			fields:  fields{http.StatusOK, "OK", 0, ""},
			args:    args{url: "/path"},
			wantErr: false,
		},
		{
			name:    "200 with no body -> ErrEmptyBody",
			fields:  fields{http.StatusOK, "", 0, ""},
			args:    args{url: "/path"},
			wantErr: true,
			wantIs:  errs.ErrEmptyBody,
		},
		//{
		//	name:    "200 with body containing 'not found' -> ErrNotFound",
		//	fields:  fields{http.StatusOK, "blah not found blah", 0, ""},
		//	args:    args{url: "/path"},
		//	wantErr: true,
		//	wantIs:  errs.ErrNotFound,
		//},
		{
			name:    "404 with body -> ErrNotFound",
			fields:  fields{http.StatusNotFound, "blah not found blah", 0, ""},
			args:    args{url: "/path"},
			wantErr: true,
			wantIs:  errs.ErrNotFound,
		},
		{
			name:    "410 with body -> ErrNotFound",
			fields:  fields{http.StatusGone, "blah not found blah", 0, ""},
			args:    args{url: "/path"},
			wantErr: true,
			wantIs:  errs.ErrNotFound,
		},
		{
			name:    "204 No Content -> ErrEmptyBody",
			fields:  fields{http.StatusNoContent, "", 0, ""},
			args:    args{url: "/nocontent"},
			wantErr: true,
			wantIs:  errs.ErrEmptyBody,
		},
		{
			name:   "500 -> we ignore",
			fields: fields{http.StatusInternalServerError, "oops", 0, ""},
			args:   args{url: "/err"},
		},
		{
			name:   "429 -> we skip",
			fields: fields{http.StatusTooManyRequests, "oops", 0, ""},
			args:   args{url: "/err"},
		},
		{
			name:   "401 -> we skip",
			fields: fields{http.StatusUnauthorized, "oops", 0, ""},
			args:   args{url: "/err"},
		},
		{
			name:          "Network timeout -> non-sentinel error",
			fields:        fields{http.StatusOK, "OK but too slow", 200 * time.Millisecond, ""},
			args:          args{url: "/slow"},
			wantErr:       true,
			wantIs:        nil,
			timeoutClient: true,
		},
		//{
		//	name:    "Body contains 'does not contain the path' -> ErrNotFound",
		//	fields:  fields{http.StatusOK, "repository exists but does not contain the path", 0, ""},
		//	args:    args{url: "/missing-path"},
		//	wantErr: true,
		//	wantIs:  errs.ErrNotFound,
		//},
		//{
		//	name:    "Uppercase 'NOT FOUND' is not matched (case sensitive) -> no error",
		//	fields:  fields{http.StatusOK, "NOT FOUND", 0, ""},
		//	args:    args{url: "/caps"},
		//	wantErr: true,
		//	wantIs:  errs.ErrNotFound,
		//},
		{
			name: "Large body with 'not found' after 10KB is ignored -> no error",
			fields: fields{
				status: http.StatusOK,
				body:   strings.Repeat("A", 11000) + " not found", // beyond the 4096 read limit
			},
			args:    args{url: "/long"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// generate a test server so we can capture and inspect the request
			var hit bool
			testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				hit = true
				if tt.fields.loc != "" {
					res.Header().Set("Location", tt.fields.loc)
				}
				if tt.fields.sleep > 0 {
					time.Sleep(tt.fields.sleep)
				}
				res.WriteHeader(tt.fields.status)

				_, _ = res.Write([]byte(tt.fields.body))
			}))
			t.Cleanup(testServer.Close)

			proc := New(time.Second, nil)
			// Make sure we don't follow redirects (aligns with your policy).
			proc.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
			if tt.timeoutClient {
				proc.httpClient.Timeout = 50 * time.Millisecond
			}

			err := proc.Process(context.TODO(), testServer.URL+tt.args.url, "")
			// If we expect short-circuit, ensure server wasn't hit.
			if tt.expectNoRequest && hit {
				t.Fatalf("expected no HTTP request to be made, but handler was hit")
			}

			if (err != nil) != tt.wantErr {
				t.Fatalf("Process() expects error '%v', got %v", tt.wantIs, err)
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

			if err.Error() != tt.wantIs.Error() {
				t.Fatalf("Got error message:\n %s\n want:\n %s", err.Error(), tt.wantIs.Error())
			}
		})
	}
}

func TestLinkProcessor_urlShouldBeIgnored(t *testing.T) {
	type fields struct {
		ignoredDomains []string
	}
	type args struct {
		url string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "no ignored domains",
			fields: fields{},
			args:   args{url: "https://github.com"},
			want:   false,
		},
		{
			name:   "no ignored domains",
			fields: fields{ignoredDomains: []string{"https://github.com"}},
			args:   args{url: "https://github.com"},
			want:   true,
		},
		{
			name:   "ignored https://github.com",
			fields: fields{ignoredDomains: []string{"https://github.com"}},
			args:   args{url: "https://github.com"},
			want:   true,
		},
		{
			name:   "ignored github",
			fields: fields{ignoredDomains: []string{"github"}},
			args:   args{url: "https://github.com"},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := &LinkProcessor{
				ignoredDomains: tt.fields.ignoredDomains,
			}
			if got := proc.urlShouldBeIgnored(tt.args.url); got != tt.want {
				t.Errorf("urlShouldBeIgnored() = %v, want %v", got, tt.want)
			}
		})
	}
}
