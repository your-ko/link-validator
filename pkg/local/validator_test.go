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

	proc := New(nil)

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
		fileNameTested string // file name being tested
		fileName       string // test file where the test link points to
		dirName        string // test dir where the test link points to
		//customContent  string // if non-empty, write this content instead of default into the test file
	}
	type args struct {
		link string // simulates a link found in a file
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantIs  error
	}{
		{
			name: "existing file at root",
			args: args{link: "README.md"},
			fields: fields{
				fileNameTested: "README.md",
				fileName:       "README.md",
			},
		},
		{
			name: "existing file inside some dir",
			args: args{link: "test/README.md"},
			fields: fields{
				fileNameTested: "README.md",
				fileName:       "test/README.md"},
		},
		{
			name:   "existing file inside some dir with a header",
			args:   args{link: "test/README.md#header1"},
			fields: fields{fileNameTested: "README.md", fileName: "test/README.md"},
		},
		//{
		//	name:    "existing file inside some dir with a non-existent header",
		//	args:    args{link: "test/qqq.md#header2"},
		//	fields:  fields{fileName: "test/qqq.md"},
		//	wantErr: true,
		//	wantIs:  errs.ErrNotFound,
		//},
		{
			name:   "existing dir in root",
			args:   args{link: "test"},
			fields: fields{fileNameTested: "README.md", dirName: "test"},
		},
		{
			name:   "existing dir in deeper",
			args:   args{link: "test/test"},
			fields: fields{fileNameTested: "README.md", dirName: "test/test"},
		},
		{
			name:    "non-existing dir",
			args:    args{link: "test/test"},
			fields:  fields{fileNameTested: "README.md", dirName: "test1"},
			wantErr: true,
			wantIs:  errs.ErrNotFound,
		},
		{
			name:    "non-existing file",
			args:    args{link: "doesnt_exists.md"},
			fields:  fields{fileNameTested: "README.md", fileName: "README.md"},
			wantErr: true,
			wantIs:  errs.ErrNotFound,
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
			fields:  fields{fileNameTested: "README.md", dirName: "dir"},
			wantErr: true,
			wantIs:  errs.ErrHeadingLinkToDir,
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
		//	name: "case-sensitive header mismatch -> ErrNotFound",
		//	args: args{link: "case.md#header1"},
		//	fields: fields{
		//		//TODO look closer into case senvisive
		//		fileName:      "case.md",
		//		customContent: "# Header1\n", // capital H
		//	},
		//	wantErr: true,
		//	wantIs:  errs.ErrNotFound,
		//},
		//{
		//	name: "percent-encoded fragment in link does not match raw text -> ErrNotFound",
		//	args: args{link: "enc.md#header%201"},
		//	fields: fields{
		//		fileName:      "enc.md",
		//		customContent: "# header 1\n",
		//	},
		//	wantErr: true,
		//	wantIs:  errs.ErrNotFound,
		//},
		{
			name:    "empty fragment (file.md#) treated as incorrect",
			args:    args{link: "emptyfrag.md#"},
			fields:  fields{fileNameTested: "README.md", fileName: "README.md"},
			wantErr: true,
			wantIs:  errs.ErrEmptyHeading,
		},
	}

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

	proc := New(zap.NewNop())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fields.fileName != "" {
				mkFile(tt.fields.fileName)
			}
			if tt.fields.dirName != "" {
				mkDir(tt.fields.dirName)
			}
			defer cleanUp(tt.fields)

			err := proc.Process(context.Background(), tt.args.link, fmt.Sprintf("%s/%s", tmp, tt.fields.fileNameTested))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Process(%q) expects error %v, got = '%v'", tt.args.link, tt.wantErr, err)
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

			expected := fmt.Sprintf("%s. Incorrect link: '%s/%s'", tt.wantIs, tmp, tt.args.link)
			if err.Error() != expected {
				t.Fatalf("Got error message:\n %s\n want:\n %s", err.Error(), expected)
			}
		})
	}
}

const content = `
test
# header1
test
## header2
test
`
