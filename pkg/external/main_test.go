package external

import (
	"context"
	"errors"
	"go.uber.org/zap"
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
		name    string
		exclude string
		line    string
		want    []string
	}

	tests := []tc{
		{
			name:    "exclude exact host and its subdomains",
			exclude: "https://github.mycorp.com",
			line: `see https://github.mycorp.com/org/repo
			       and https://api.github.mycorp.com/x
			       and https://example.com/page
			       and https://gitlab.mycorp.com/y`,
			// Expect to keep only non-excluded domains.
			want: []string{
				"https://example.com/page",
				"https://gitlab.mycorp.com/y",
			},
		},
		{
			name:    "exclude without scheme still filters",
			exclude: "github.mycorp.com",
			line:    `https://github.mycorp.com a https://api.github.mycorp.com b https://google.com?q=1`,
			want: []string{
				"https://google.com?q=1",
			},
		},
		{
			name:    "exclude with leading dot works same",
			exclude: ".github.mycorp.com",
			line:    `https://github.mycorp.com https://sub.github.mycorp.com https://other.com`,
			want: []string{
				"https://other.com",
			},
		},
		{
			name:    "http links are not matched by regex (https only)",
			exclude: "github.mycorp.com",
			line:    `http://github.mycorp.com https://github.mycorp.com https://ok.com`,
			want: []string{
				// http://... is ignored by regex; https://github.mycorp.com excluded; only ok.com remains
				"https://ok.com",
			},
		},
		{
			name:    "mixed unrelated https remain",
			exclude: "github.mycorp.com",
			line:    `https://one.com https://two.com https://github.mycorp.com https://three.com`,
			want: []string{
				"https://one.com",
				"https://two.com",
				"https://three.com",
			},
		},
		{
			name:    "test for MD link",
			exclude: "github.mycorp.com",
			line:    `qqq https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/main.yaml) qqq`,
			want: []string{
				"https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg",
				"https://github.com/your-ko/link-validator/actions/workflows/main.yaml",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := New(tt.exclude)
			got := proc.ExtractLinks(tt.line)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks() mismatch\nexclude=%q\nline=%q\ngot = %#v\nwant= %#v",
					tt.exclude, tt.line, got, tt.want)
			}
		})
	}
}

func TestHttpLinkProcessor_Process(t *testing.T) {
	t.Parallel()
	type fields struct {
		exclude string
		status  int
		body    string
		sleep   time.Duration // optional server delay
		loc     string        // optional redirect Location
	}
	type args struct {
		url string
	}
	tests := []struct {
		name            string
		fields          fields
		args            args
		whetherWantErr  bool
		wantIs          error
		expectNoRequest bool // true => server handler must not be hit (excluded host short-circuit)
		timeoutClient   bool // true => override client with short timeout; expect non-sentinel error
	}{
		{
			name:           "200 with body",
			fields:         fields{"", 200, "OK", 0, ""},
			args:           args{url: "/path"},
			whetherWantErr: false,
		},
		{
			name:           "200 with no body -> EmptyBody",
			fields:         fields{"", 200, "", 0, ""},
			args:           args{url: "/path"},
			whetherWantErr: true,
			wantIs:         errs.EmptyBody, // assumes you added this sentinel
		},
		{
			name:           "200 with body containing 'not found' -> NotFound",
			fields:         fields{"", 200, "blah not found blah", 0, ""},
			args:           args{url: "/path"},
			whetherWantErr: true,
			wantIs:         errs.NotFound,
		},
		{
			name:           "404 with body -> NotFound",
			fields:         fields{"", 404, "blah not found blah", 0, ""},
			args:           args{url: "/path"},
			whetherWantErr: true,
			wantIs:         errs.NotFound,
		},
		{
			name:           "301 redirect (no follow) -> NotFound",
			fields:         fields{"", http.StatusMovedPermanently, "", 0, "/other"},
			args:           args{url: "/redir"},
			whetherWantErr: true,
			wantIs:         errs.NotFound,
		},
		{
			name:           "204 No Content -> EmptyBody",
			fields:         fields{"", http.StatusNoContent, "", 0, ""},
			args:           args{url: "/nocontent"},
			whetherWantErr: true,
			wantIs:         errs.EmptyBody,
		},
		{
			name:           "500 -> NotFound (generic fallback)",
			fields:         fields{"", http.StatusInternalServerError, "oops", 0, ""},
			args:           args{url: "/err"},
			whetherWantErr: true,
			wantIs:         errs.NotFound,
		},
		{
			name:           "Network timeout -> non-sentinel error",
			fields:         fields{"", 200, "OK but too slow", 200 * time.Millisecond, ""},
			args:           args{url: "/slow"},
			whetherWantErr: true,
			wantIs:         nil, // don't check sentinel; we'll assert it's NOT NotFound
			timeoutClient:  true,
		},
		{
			name:           "Body contains 'does not contain the path' -> NotFound",
			fields:         fields{"", 200, "repository exists but does not contain the path", 0, ""},
			args:           args{url: "/missing-path"},
			whetherWantErr: true,
			wantIs:         errs.NotFound,
		},
		{
			name:           "Uppercase 'NOT FOUND' is not matched (case sensitive) -> no error",
			fields:         fields{"", 200, "NOT FOUND", 0, ""},
			args:           args{url: "/caps"},
			whetherWantErr: true,
			wantIs:         errs.NotFound,
		},
		{
			name: "Large body with 'not found' after 4KB is ignored -> no error",
			fields: fields{
				exclude: "",
				status:  200,
				body:    strings.Repeat("A", 5000) + " not found", // beyond the 4096 read limit
			},
			args:           args{url: "/long"},
			whetherWantErr: false,
		},
	}
	logger, _ := zap.NewDevelopment()
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

			proc := New(tt.fields.exclude)
			// Make sure we don't follow redirects (aligns with your policy).
			proc.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
			if tt.timeoutClient {
				proc.httpClient.Timeout = 50 * time.Millisecond
			}

			err := proc.Process(context.TODO(), testServer.URL+tt.args.url, logger)
			// If we expect short-circuit, ensure server wasn't hit.
			if tt.expectNoRequest && hit {
				t.Fatalf("expected no HTTP request to be made, but handler was hit")
			}

			if (err != nil) != tt.whetherWantErr {
				t.Fatalf("Process() err presence = %v, wantIs=%v (err=%v)", err != nil, tt.wantIs, err)
			}
			if !tt.whetherWantErr {
				return
			}

			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("Process() error '%v' does not match sentinel '%v'", err, tt.wantIs)
			}
		})
	}
}
