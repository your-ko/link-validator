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
		fileName      string
		dirName       string
		customContent string // if non-empty, write this content instead of default
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
			args:   args{link: "README.md"},
			fields: fields{fileName: "README.md"},
		},
		{
			name:   "existing file inside some dir",
			args:   args{link: "test/README.md"},
			fields: fields{fileName: "test/README.md"},
		},
		{
			name:   "existing file inside some dir with a header",
			args:   args{link: "test/README.md#header1"},
			fields: fields{fileName: "test/README.md"},
		},
		//{
		//	name:    "existing file inside some dir with a non-existent header",
		//	args:    args{link: "test/qqq.md#header2"},
		//	fields:  fields{fileName: "test/qqq.md"},
		//	wantErr: true,
		//	wantIs:  errs.NotFound,
		//},
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
			wantIs:  errs.NotFound,
		},
		{
			name:    "non-existing file",
			args:    args{link: "doesnt_exists.md"},
			fields:  fields{fileName: "README.md"},
			wantErr: true,
			wantIs:  errs.NotFound,
		},

		//{
		//	name:            "multiple # fragments -> incorrect link error",
		//	args:            args{link: "test/multi.md#h1#h2"},
		//	fields:          fields{fileName: "test/multi.md"},
		//	wantErr:         true,
		//	wantErrContains: "Contains more than one #",
		//},
		{
			name:    "directory with header fragment -> incorrect link error",
			args:    args{link: "dir#header"},
			fields:  fields{dirName: "dir"},
			wantErr: true,
			wantIs:  errs.HeadingLinkToDir,
		},
		//{
		//	name: "header line with multiple spaces after # should match",
		//	args: args{link: "spaced.md#header1"},
		//	fields: fields{
		//		fileName:      "spaced.md",
		//		customContent: "intro\n#    header1\nrest\n",
		//	},
		//},
		//{
		//	name: "case-sensitive header mismatch -> NotFound",
		//	args: args{link: "case.md#header1"},
		//	fields: fields{
		//		//TODO look closer into case senvisive
		//		fileName:      "case.md",
		//		customContent: "# Header1\n", // capital H
		//	},
		//	wantErr: true,
		//	wantIs:  errs.NotFound,
		//},
		//{
		//	name: "percent-encoded fragment in link does not match raw text -> NotFound",
		//	args: args{link: "enc.md#header%201"},
		//	fields: fields{
		//		fileName:      "enc.md",
		//		customContent: "# header 1\n",
		//	},
		//	wantErr: true,
		//	wantIs:  errs.NotFound,
		//},
		{
			name:    "empty fragment (file.md#) treated as incorrect",
			args:    args{link: "emptyfrag.md#"},
			fields:  fields{fileName: "README.md"},
			wantErr: true,
			wantIs:  errs.EmptyHeading,
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
				t.Fatalf("expected \n errors.Is(err, %v) to be true; \n got err=%v", tt.wantIs, err)
			}

			expected := fmt.Sprintf("%s. incorrect link: '%s/%s'", tt.wantIs, tmp, tt.args.link)
			if err.Error() != expected {
				t.Fatalf("Got error message:\n %s\n want:\n %s", err.Error(), expected)
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
