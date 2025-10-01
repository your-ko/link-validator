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
	tmp := t.TempDir()

	type fields struct {
		fileName string
		dirName  string
	}
	type args struct {
		link string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantIs  error
	}{
		{
			name:   "existing file at root",
			args:   args{link: "qqq.md"},
			fields: fields{fileName: "qqq.md"},
		},
		{
			name:   "existing file inside some dir",
			args:   args{link: "test/qqq.md"},
			fields: fields{fileName: "test/qqq.md"},
		},
		{
			name:   "existing file inside some dir with a header",
			args:   args{link: "test/qqq.md#header1"},
			fields: fields{fileName: "test/qqq.md"},
		},
		{
			name:    "existing file inside some dir with a non-existent header",
			args:    args{link: "test/qqq.md#header2"},
			fields:  fields{fileName: "test/qqq.md"},
			wantErr: true,
			wantIs:  errs.NewNotFound(fmt.Sprintf("%s/%s", tmp, "test/qqq.md#header2")),
		},
		{
			name:   "existing dir in root",
			args:   args{link: "test"},
			fields: fields{dirName: "test"},
		},
		{
			name:   "existing dir in deeper",
			args:   args{link: "test/test"},
			fields: fields{dirName: "test/test"},
		},
		{
			name:    "non-existing dir",
			args:    args{link: "test/test"},
			fields:  fields{dirName: "test1"},
			wantErr: true,
			wantIs:  errs.NewNotFound(fmt.Sprintf("%s/%s", tmp, "test/test")),
		},
	}

	logger := zap.NewNop()

	mkDir := func(rel string) {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	mkFile := func(rel string) {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	cleanUp := func(test fields) {
		if test.fileName != "" {
			err := os.Remove(fmt.Sprintf("%s/%s", tmp, test.fileName))
			if err != nil {
				t.Fatalf("cleanup: %v", err)
			}
		}
		if test.dirName != "" {
			err := os.Remove(fmt.Sprintf("%s/%s", tmp, test.dirName))
			if err != nil {
				t.Fatalf("cleanup: %v", err)
			}
		}
	}

	proc := &LinkProcessor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fields.fileName != "" {
				mkFile(tt.fields.fileName)
			}
			if tt.fields.dirName != "" {
				mkDir(tt.fields.dirName)
			}
			defer cleanUp(tt.fields)

			err := proc.Process(context.Background(), fmt.Sprintf("%s/%s", tmp, tt.args.link), logger)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Process(%q) error presence = %v, want %v (err = %v)",
					tt.args.link, err != nil, tt.wantErr, err)
			}
			if !tt.wantErr {
				return
			}

			// If a sentinel is specified, ensure errors.Is matches it.
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected errors.Is(err, %v) to be true; got err=%v", tt.wantIs, err)
			}

			if errors.Is(err, errs.NotFound) {
				expected := fmt.Sprintf("%s/%s", tmp, tt.args.link)
				if err.Error() != expected {
					t.Fatalf("NotFoundError.Error() = %q, want %q", err.Error(), expected)
				}
			}
		})
	}
}

var content = `
test
# header1
test
## header2
test
`
