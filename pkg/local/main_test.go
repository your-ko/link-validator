package local

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"link-validator/pkg/errs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLinkProcessor_ExtractLinks_LocalOnly(t *testing.T) {
	t.Parallel()

	proc := New()

	type tc struct {
		name string
		line string
		want []string // full markdown link tokens, e.g. "[txt](../a.md)"
	}

	tests := []tc{
		{
			name: "bare filename allowed",
			line: `build with [make](Makefile)`,
			want: []string{
				"Makefile",
			},
		},
		{
			name: "relative nested path allowed",
			line: `see [guide](docs/guide.md) and [spec](api/v1/spec.md)`,
			want: []string{
				"docs/guide.md",
				"api/v1/spec.md",
			},
		},
		{
			name: "./ and ../ prefixes allowed (any depth)",
			line: `open [here](./README.md) then [up](../CONTRIBUTING.md) and [up2](../../docs/ref.md)`,
			want: []string{
				"./README.md",
				"../CONTRIBUTING.md",
				"../../docs/ref.md",
			},
		},
		{
			name: "fragment after local path is allowed",
			line: `jump to [section](docs/guide.md#install) and [another](../a/b.md#L10-L20)`,
			want: []string{
				"docs/guide.md#install",
				"../a/b.md#L10-L20",
			},
		},
		{
			name: "external links are ignored (https, http, mailto, protocol-relative, absolute path)",
			line: `ext1 [g](https://google.com) ext2 [e](http://example.com) mail [m](mailto:me@ex.com) proto [p](//cdn.example.com/x) abs [r](/root/readme.md) local [l](docs/ok.md)`,
			want: []string{
				"docs/ok.md",
			},
		},
		{
			name: "multiple locals mixed with externals",
			line: `[one](a.md) [two](https://ex.com) [three](b/c.md) [four](mailto:x@y) [five](../d/e.md)`,
			want: []string{
				"a.md",
				"b/c.md",
				"../d/e.md",
			},
		},
		{
			name: "no matches",
			line: `nothing here: https://example.com and mailto:me@example.com`,
			want: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proc.ExtractLinks(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks mismatch\nline=%q\ngot = %#v\nwant= %#v", tt.line, got, tt.want)
			}
		})
	}
}

func TestLinkProcessor_Process(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()

	// Create some fixtures
	mkFile := func(rel string) {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	mkDir := func(rel string) {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	mkFile("a.md")
	mkFile("docs/readme.md")
	mkDir("somedir")

	logger := zap.NewNop()
	proc := &LinkProcessor{
		path: tmp,
	}

	type tc struct {
		name    string
		url     string
		wantErr bool
		wantIs  error // sentinel to check with errors.Is (e.g., errs.NotFound), nil => don’t check
	}
	tests := []tc{
		{
			name:    "existing file at root -> nil",
			url:     "a.md",
			wantErr: false,
		},
		{
			name:    "nested existing file -> nil",
			url:     "docs/readme.md",
			wantErr: false,
		},
		{
			name:    "missing file -> NotFound",
			url:     "missing.md",
			wantErr: true,
			wantIs:  errs.NotFound,
		},
		{
			name:    "path points to a directory -> non-NotFound error",
			url:     "somedir", // ReadFile on a directory returns an error (EISDIR on Unix)
			wantErr: true,
			// wantIs nil: we assert it's NOT NotFound below
		},
		{
			name:    "relative current-dir style -> nil",
			url:     "./a.md",
			wantErr: false,
		},
		{
			name:    "relative nested style -> nil",
			url:     "./docs/readme.md",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := proc.Process(context.Background(), tt.url, logger)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Process(%q) error presence = %v, want %v (err = %v)",
					tt.url, err != nil, tt.wantErr, err)
			}
			if !tt.wantErr {
				return
			}

			// If a sentinel is specified, ensure errors.Is matches it.
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected errors.Is(err, %v) to be true; got err=%v", tt.wantIs, err)
			}

			// For the directory case (or any non-sentinel case), ensure it's NOT mapped to NotFound.
			if tt.wantIs == nil && errors.Is(err, errs.NotFound) {
				t.Fatalf("unexpected mapping to errs.NotFound for url=%q; err=%v", tt.url, err)
			}

			// Optional: if it's NotFound, ensure the error string contains the constructed filename
			if errors.Is(err, errs.NotFound) {
				expected := fmt.Sprintf("%s/%s", proc.path, tt.url) // matches the function’s join logic
				if err.Error() != expected {
					t.Fatalf("NotFoundError.Error() = %q, want %q", err.Error(), expected)
				}
			}
		})
	}
}
