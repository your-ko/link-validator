package link_validator

import (
	"crypto/rand"
	"encoding/base64"
	"link-validator/pkg/config"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func GetRndName() (string, error) {
	b := make([]byte, 6) // 6 bytes -> 8 base64 chars
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil // [A-Za-z0-9-_], length 8
}

func TestMatchesFileMask(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		masks    []string
		matched  bool
	}{
		{
			name:     "matches single mask",
			filename: "readme.md",
			masks:    []string{"*.md"},
			matched:  true,
		},
		{
			name:     "matches multiple masks",
			filename: "test.txt",
			masks:    []string{"*.md", "*.txt", "*.go"},
			matched:  true,
		},
		{
			name:     "no match",
			filename: "validator.go",
			masks:    []string{"*.md", "*.txt"},
			matched:  false,
		},
		{
			name:     "empty masks",
			filename: "any.file",
			masks:    []string{},
			matched:  false,
		},
		{
			name:     "complex pattern match",
			filename: "test_file.md",
			masks:    []string{"test_*.md"},
			matched:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFileMask(tt.filename, tt.masks)
			if got != tt.matched {
				t.Errorf("matchesFileMask(%q, %v) = %v, want %v",
					tt.filename, tt.masks, got, tt.matched)
			}
		})
	}
}

func TestLinkValidador_GetFiles(t *testing.T) {
	type fields struct {
		dirName   string
		fileNames []string
	}
	type args struct {
		config *config.Config
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name:    "File list is specified",
			args:    args{config: &config.Config{Files: []string{"README.md", "Makefile"}, FileMasks: []string{"*.md"}}},
			want:    []string{"README.md"},
			wantErr: false,
		},
		{
			name: "File list is not specified, walk over repo",
			args: args{config: &config.Config{FileMasks: []string{"*.md"}}},
			fields: fields{
				fileNames: []string{"README.md", "Makefile", "action.yml", "Dockerfile"},
			},
			want:    []string{"README.md"},
			wantErr: false,
		},
	}
	tmpName, err := GetRndName()
	if err != nil {
		t.Fatalf("can't create tmp dir: %s", err)
	}
	tmp := filepath.Join(os.TempDir(), tmpName)
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
		if len(test.fileNames) != 0 {
			for _, f := range test.fileNames {
				err := os.Remove(filepath.Join(tmp, f))
				if err != nil && !os.IsNotExist(err) {
					t.Fatalf("cleanup file: %v", err)
				}
			}
		}
		err := os.RemoveAll(filepath.Join(tmp))
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("cleanup dir: %v", err)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.fields.fileNames) != 0 {
				mkDir(tt.fields.dirName)
				for _, f := range tt.fields.fileNames {
					mkFile(f)
				}
				// because I test in tmp dir, I can't set up lookup path in the config
				tt.args.config.LookupPath = tmp
				// Update expected paths to include full temporary directory path
				if len(tt.want) > 0 && !filepath.IsAbs(tt.want[0]) {
					for i, wantFile := range tt.want {
						tt.want[i] = filepath.Join(tmp, wantFile)
					}
				}
			}
			t.Cleanup(func() {
				cleanUp(tt.fields)
			})

			// Create file processing pipeline for test
			fileProcessor := ProcessFilesPipeline(
				WalkDirectoryProcessor(tt.args.config),
				IncludeExplicitFilesProcessor(tt.args.config.Files),
				FilterByMaskProcessor(tt.args.config.FileMasks),
				ExcludePathsProcessor(tt.args.config.ExcludePath),
			)
			v := &LinkValidador{nil, fileProcessor}
			got, err := v.GetFiles()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetFiles() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_subtraction(t *testing.T) {
	type args struct {
		left  []string
		right []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty left slice",
			args: args{
				left:  []string{},
				right: []string{"a", "b"},
			},
			want: []string{},
		},
		{
			name: "empty right slice",
			args: args{
				left:  []string{"a", "b", "c"},
				right: []string{},
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "both slices empty",
			args: args{
				left:  []string{},
				right: []string{},
			},
			want: []string{},
		},
		{
			name: "no intersection - all elements remain",
			args: args{
				left:  []string{"a", "b", "c"},
				right: []string{"x", "y", "z"},
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "partial intersection - some elements removed",
			args: args{
				left:  []string{"a", "b", "c", "d"},
				right: []string{"b", "d", "x"},
			},
			want: []string{"a", "c"},
		},
		{
			name: "complete intersection - all elements removed",
			args: args{
				left:  []string{"a", "b", "c"},
				right: []string{"a", "b", "c"},
			},
			want: []string{},
		},
		{
			name: "right slice larger than left",
			args: args{
				left:  []string{"a", "b"},
				right: []string{"a", "b", "c", "d", "e"},
			},
			want: []string{},
		},
		{
			name: "left slice larger than right",
			args: args{
				left:  []string{"a", "b", "c", "d", "e"},
				right: []string{"b", "d"},
			},
			want: []string{"a", "c", "e"},
		},
		{
			name: "duplicate elements in left slice",
			args: args{
				left:  []string{"a", "b", "a", "c", "b"},
				right: []string{"a"},
			},
			want: []string{"b", "c"}, // Note: duplicates are removed due to map usage
		},
		{
			name: "duplicate elements in right slice",
			args: args{
				left:  []string{"a", "b", "c"},
				right: []string{"a", "a", "b", "b"},
			},
			want: []string{"c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subtraction(tt.args.left, tt.args.right)
			// Since the order of elements in the result is not deterministic due to map iteration,
			// we need to compare the sets rather than the slices directly
			if !equalSets(got, tt.want) {
				t.Errorf("subtraction() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to compare two slices as sets (ignoring order)
func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	setA := make(map[string]bool, len(a))
	for _, v := range a {
		setA[v] = true
	}

	for _, v := range b {
		if !setA[v] {
			return false
		}
	}

	return true
}

func TestExcludePathsProcessor(t *testing.T) {
	type testCase struct {
		name         string
		excludePaths []string
		inputFiles   []string
		wantFiles    []string
		wantErr      bool
	}

	tests := []testCase{
		{
			name:         "empty exclude paths - returns all files",
			excludePaths: []string{},
			inputFiles:   []string{"file1.md", "file2.go", "file3.txt"},
			wantFiles:    []string{"file1.md", "file2.go", "file3.txt"},
			wantErr:      false,
		},
		{
			name:         "nil exclude paths - returns all files",
			excludePaths: nil,
			inputFiles:   []string{"file1.md", "file2.go", "file3.txt"},
			wantFiles:    []string{"file1.md", "file2.go", "file3.txt"},
			wantErr:      false,
		},
		{
			name:         "empty input files - returns empty",
			excludePaths: []string{"vendor/", "node_modules/"},
			inputFiles:   []string{},
			wantFiles:    []string{},
			wantErr:      false,
		},
		{
			name:         "exclude single file",
			excludePaths: []string{"README.md"},
			inputFiles:   []string{"README.md", "main.go", "Dockerfile"},
			wantFiles:    []string{"main.go", "Dockerfile"},
			wantErr:      false,
		},
		{
			name:         "exclude multiple files",
			excludePaths: []string{"vendor/lib.go", "test/main_test.go"},
			inputFiles:   []string{"src/main.go", "vendor/lib.go", "docs/README.md", "test/main_test.go"},
			wantFiles:    []string{"src/main.go", "docs/README.md"},
			wantErr:      false,
		},
		{
			name:         "exclude directory paths",
			excludePaths: []string{"vendor/", "node_modules/", ".git/"},
			inputFiles:   []string{"src/main.go", "vendor/", "docs/README.md", "node_modules/", ".git/"},
			wantFiles:    []string{"src/main.go", "docs/README.md"},
			wantErr:      false,
		},
		{
			name:         "no matches - returns all files",
			excludePaths: []string{"nonexistent.txt", "missing/"},
			inputFiles:   []string{"src/main.go", "docs/README.md", "Dockerfile"},
			wantFiles:    []string{"src/main.go", "docs/README.md", "Dockerfile"},
			wantErr:      false,
		},
		{
			name:         "exclude all files",
			excludePaths: []string{"file1.md", "file2.go", "file3.txt"},
			inputFiles:   []string{"file1.md", "file2.go", "file3.txt"},
			wantFiles:    []string{},
			wantErr:      false,
		},
		{
			name:         "exclude paths with duplicates in input",
			excludePaths: []string{"duplicate.md"},
			inputFiles:   []string{"unique.go", "duplicate.md", "unique.go", "another.txt", "duplicate.md"},
			wantFiles:    []string{"unique.go", "another.txt"}, // duplicates removed by subtraction
			wantErr:      false,
		},
		{
			name:         "exclude paths larger than input",
			excludePaths: []string{"a.md", "b.go", "c.txt", "d.yml", "e.json"},
			inputFiles:   []string{"a.md", "b.go"},
			wantFiles:    []string{},
			wantErr:      false,
		},
		{
			name:         "complex file paths exclusion",
			excludePaths: []string{"build/output/", "dist/bundle.js", "coverage/report.html"},
			inputFiles: []string{
				"src/components/Header.tsx",
				"src/utils/helpers.ts",
				"build/output/",
				"dist/bundle.js",
				"coverage/report.html",
				"package.json",
			},
			wantFiles: []string{"src/components/Header.tsx", "src/utils/helpers.ts", "package.json"},
			wantErr:   false,
		},
		{
			name:         "exclude with absolute and relative paths",
			excludePaths: []string{"/absolute/path/file.go", "relative/path/file.md"},
			inputFiles: []string{
				"src/main.go",
				"/absolute/path/file.go",
				"relative/path/file.md",
				"docs/README.md",
			},
			wantFiles: []string{"src/main.go", "docs/README.md"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := ExcludePathsProcessor(tt.excludePaths)
			got, err := processor(tt.inputFiles)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExcludePathsProcessor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Since subtraction function uses map internally, order is not deterministic
			// Use the same equalSets helper function we created for subtraction tests
			if !equalSets(got, tt.wantFiles) {
				t.Errorf("ExcludePathsProcessor() = %v, want %v", got, tt.wantFiles)
			}
		})
	}
}
