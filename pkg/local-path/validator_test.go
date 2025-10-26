package local_path

import (
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

func TestLinkProcessor_parseLink(t *testing.T) {
	type args struct {
		link string
	}
	tests := []struct {
		name       string
		args       args
		wantPath   string
		wantHeader string
		wantErr    bool
		wantIs     error
	}{
		{
			name:       "normal link",
			args:       args{"./filename"},
			wantPath:   "./filename",
			wantHeader: "",
			wantErr:    false,
		},
		{
			name:    "link with the empty anchor",
			args:    args{"./filename#"},
			wantErr: true,
			wantIs:  errs.ErrEmptyAnchor,
		},
		{
			name:       "normal link with an anchor",
			args:       args{"./filename#qqq"},
			wantPath:   "./filename",
			wantHeader: "qqq",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := New(zap.NewNop())
			gotPath, gotHeader, err := proc.parseLink(tt.args.link)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLink() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotPath != tt.wantPath {
				t.Errorf("parseLink():\n gotPath: %v,\n    want: %v", gotPath, tt.wantPath)
			}
			if gotHeader != tt.wantHeader {
				t.Errorf("parseLink():\n gotHeader: %v,\n     want: %v", gotHeader, tt.wantHeader)
			}

			if tt.wantIs == nil {
				return
			}

			// If a sentinel is specified, ensure errors.Is matches it.
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected \n errors.Is(err, %v) to be true; \n got err=%v", tt.wantIs, err)
			}

			expected := fmt.Sprintf("%s. Incorrect link: '%s'", tt.wantIs, tt.args.link)
			if err.Error() != expected {
				t.Fatalf("Got error message:\n %s\n want:\n %s", err.Error(), expected)
			}

		})
	}
}

func TestLinkProcessor_resolveTargetPath(t *testing.T) {
	type args struct {
		linkPath     string
		testFileName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple filename in root",
			args: args{
				linkPath:     "LICENSE",
				testFileName: "README.md",
			},
			want: "LICENSE",
		},
		{
			name: "simple filename in root with with ./ prefix",
			args: args{
				linkPath:     "./LICENSE",
				testFileName: "README.md",
			},
			want: "LICENSE",
		},
		{
			name: "simple filename in path",
			args: args{
				linkPath:     "guide.md",
				testFileName: "/docs/README.md",
			},
			want: "/docs/guide.md",
		},
		{
			name: "relative path with ./ prefix",
			args: args{
				linkPath:     "./guide.md",
				testFileName: "/docs/README.md",
			},
			want: "docs/guide.md",
		},
		{
			name: "relative path with ../ going up one level",
			args: args{
				linkPath:     "../README.md",
				testFileName: "/docs/guide.md",
			},
			want: "/README.md",
		},
		{
			name: "relative path with multiple ../ going up multiple levels",
			args: args{
				linkPath:     "../../CONTRIBUTING.md",
				testFileName: "/project/docs/api/README.md",
			},
			want: "/project/CONTRIBUTING.md",
		},
		{
			name: "nested path within same directory",
			args: args{
				linkPath:     "test/test.md",
				testFileName: "/docs/guide.md",
			},
			want: "/docs/test/test.md",
		},
		{
			name: "complex relative path with ./ and nested directories",
			args: args{
				linkPath:     "./images/diagram.png",
				testFileName: "/docs/guide.md",
			},
			want: "docs/images/diagram.png",
		},
		{
			name: "mixed relative path up and down",
			args: args{
				linkPath:     "../assets/style.css",
				testFileName: "/docs/guide.md",
			},
			want: "/assets/style.css",
		},
		{
			name: "test file in root, link to nested file",
			args: args{
				linkPath:     "/cmd/link-validator/main.go",
				testFileName: "/docs/README.md",
			},
			want: "/cmd/link-validator/main.go",
		},
		{
			name: "don't preserve ./ prefix for relative paths",
			args: args{
				linkPath:     "./config/settings.json",
				testFileName: "/docs/README.md",
			},
			want: "docs/config/settings.json",
		},
		{
			name: "deeply nested relative navigation",
			args: args{
				linkPath:     "../docs/guide.md",
				testFileName: "/docs/README.md",
			},
			want: "/docs/guide.md",
		},
		{
			name: "edge case: empty link path",
			args: args{
				linkPath:     "",
				testFileName: "/docs/README.md",
			},
			want: "/docs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := New(zap.NewNop())
			got := proc.resolveTargetPath(tt.args.linkPath, tt.args.testFileName)
			if got != tt.want {
				t.Errorf("resolveTargetPath(): %v,\n                                 want: %v", got, tt.want)
			}
		})
	}
}

func TestLinkProcessor_validateTarget(t *testing.T) {
	tmp := t.TempDir()

	type fields struct {
		fileName string // test file to create
		dirName  string // test directory to create
	}
	type args struct {
		targetPath string
		header     string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantIs  error
	}{
		{
			name: "existing file without header",
			fields: fields{
				fileName: "test.md",
			},
			args: args{
				targetPath: filepath.Join(tmp, "test.md"),
				header:     "",
			},
			wantErr: false,
		},
		{
			name: "existing file with header",
			fields: fields{
				fileName: "test-with-header.md",
			},
			args: args{
				targetPath: filepath.Join(tmp, "test-with-header.md"),
				header:     "section1",
			},
			wantErr: false,
		},
		{
			name: "non-existing file",
			args: args{
				targetPath: filepath.Join(tmp, "nonexistent.md"),
				header:     "",
			},
			wantErr: true,
			wantIs:  errs.ErrNotFound,
		},
		{
			name: "non-existing file with header",
			args: args{
				targetPath: filepath.Join(tmp, "nonexistent.md"),
				header:     "section1",
			},
			wantErr: true,
			wantIs:  errs.ErrNotFound,
		},
		{
			name: "existing directory without header - should pass",
			fields: fields{
				dirName: "testdir",
			},
			args: args{
				targetPath: filepath.Join(tmp, "testdir"),
				header:     "",
			},
			wantErr: false,
		},
		{
			name: "existing directory with header - should fail",
			fields: fields{
				dirName: "testdir-with-header",
			},
			args: args{
				targetPath: filepath.Join(tmp, "testdir-with-header"),
				header:     "section1",
			},
			wantErr: true,
			wantIs:  errs.ErrAnchorLinkToDir,
		},
		{
			name: "nested directory without header",
			fields: fields{
				dirName: "parent/child",
			},
			args: args{
				targetPath: filepath.Join(tmp, "parent", "child"),
				header:     "",
			},
			wantErr: false,
		},
		{
			name: "nested directory with header - should fail",
			fields: fields{
				dirName: "parent-nested/child-nested",
			},
			args: args{
				targetPath: filepath.Join(tmp, "parent-nested", "child-nested"),
				header:     "invalid",
			},
			wantErr: true,
			wantIs:  errs.ErrAnchorLinkToDir,
		},
		{
			name: "file with empty header string",
			fields: fields{
				fileName: "empty-header.md",
			},
			args: args{
				targetPath: filepath.Join(tmp, "empty-header.md"),
				header:     "",
			},
			wantErr: false,
		},
		{
			name: "file with complex header",
			fields: fields{
				fileName: "complex.md",
			},
			args: args{
				targetPath: filepath.Join(tmp, "complex.md"),
				header:     "complex-header-with-dashes_and_underscores123",
			},
			wantErr: false,
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
		if err := os.WriteFile(full, []byte("# Test Content"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	cleanUp := func(test fields) {
		if test.fileName != "" {
			err := os.Remove(filepath.Join(tmp, test.fileName))
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("cleanup file: %v", err)
			}
		}
		if test.dirName != "" {
			err := os.RemoveAll(filepath.Join(tmp, test.dirName))
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("cleanup dir: %v", err)
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
			t.Cleanup(func() {
				cleanUp(tt.fields)
			})

			err := proc.validateTarget(tt.args.targetPath, tt.args.header)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				return
			}

			if tt.wantIs == nil {
				return
			}

			// If a sentinel is specified, ensure errors.Is matches it.
			if !errors.Is(err, tt.wantIs) {
				t.Errorf("expected \n errors.Is(err, %v) to be true; \n got err=%v", tt.wantIs, err)
			}
		})
	}
}
