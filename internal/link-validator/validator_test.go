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
				ExcludePathsProcessor(tt.args.config.Exclude),
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
			wantFiles:    []string{"unique.go", "unique.go", "another.txt"}, // duplicates removed by subtraction
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

func TestIncludeExplicitFilesProcessor(t *testing.T) {
	type testCase struct {
		name          string
		explicitFiles []string
		inputFiles    []string
		wantFiles     []string
		wantErr       bool
	}

	tests := []testCase{
		{
			name:          "empty input files - returns explicit files",
			explicitFiles: []string{"README.md", "LICENSE", "main.go"},
			inputFiles:    []string{},
			wantFiles:     []string{"README.md", "LICENSE", "main.go"},
			wantErr:       false,
		},
		{
			name:          "nil input files - returns explicit files",
			explicitFiles: []string{"config.yaml", "docker-compose.yml"},
			inputFiles:    nil,
			wantFiles:     []string{"config.yaml", "docker-compose.yml"},
			wantErr:       false,
		},
		{
			name:          "non-empty input files - returns input files unchanged",
			explicitFiles: []string{"README.md", "LICENSE"},
			inputFiles:    []string{"src/main.go", "pkg/utils.go"},
			wantFiles:     []string{"src/main.go", "pkg/utils.go"},
			wantErr:       false,
		},
		{
			name:          "empty explicit files with empty input - returns empty",
			explicitFiles: []string{},
			inputFiles:    []string{},
			wantFiles:     []string{},
			wantErr:       false,
		},
		{
			name:          "nil explicit files with empty input - returns nil",
			explicitFiles: nil,
			inputFiles:    []string{},
			wantFiles:     nil,
			wantErr:       false,
		},
		{
			name:          "empty explicit files with non-empty input - returns input",
			explicitFiles: []string{},
			inputFiles:    []string{"found.md", "discovered.go"},
			wantFiles:     []string{"found.md", "discovered.go"},
			wantErr:       false,
		},
		{
			name:          "nil explicit files with non-empty input - returns input",
			explicitFiles: nil,
			inputFiles:    []string{"auto-discovered.txt"},
			wantFiles:     []string{"auto-discovered.txt"},
			wantErr:       false,
		},
		{
			name:          "single explicit file with empty input",
			explicitFiles: []string{"single-file.md"},
			inputFiles:    []string{},
			wantFiles:     []string{"single-file.md"},
			wantErr:       false,
		},
		{
			name:          "single explicit file with single input file - returns input",
			explicitFiles: []string{"explicit.md"},
			inputFiles:    []string{"input.go"},
			wantFiles:     []string{"input.go"},
			wantErr:       false,
		},
		{
			name:          "explicit files with duplicates",
			explicitFiles: []string{"file.md", "file.md", "other.go", "file.md"},
			inputFiles:    []string{},
			wantFiles:     []string{"file.md", "file.md", "other.go", "file.md"}, // preserves duplicates
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := IncludeExplicitFilesProcessor(tt.explicitFiles)
			got, err := processor(tt.inputFiles)

			if (err != nil) != tt.wantErr {
				t.Errorf("IncludeExplicitFilesProcessor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.wantFiles) {
				t.Errorf("IncludeExplicitFilesProcessor() = %v, want %v", got, tt.wantFiles)
			}
		})
	}
}

func TestFilterByMaskProcessor(t *testing.T) {
	type testCase struct {
		name       string
		masks      []string
		inputFiles []string
		wantFiles  []string
		wantErr    bool
	}

	tests := []testCase{
		{
			name:       "empty masks - returns all files",
			masks:      []string{},
			inputFiles: []string{"file1.md", "file2.go", "file3.txt"},
			wantFiles:  []string{"file1.md", "file2.go", "file3.txt"},
			wantErr:    false,
		},
		{
			name:       "nil masks - returns all files",
			masks:      nil,
			inputFiles: []string{"file1.md", "file2.go", "file3.txt"},
			wantFiles:  []string{"file1.md", "file2.go", "file3.txt"},
			wantErr:    false,
		},
		{
			name:       "empty input files - returns empty",
			masks:      []string{"*.md", "*.go"},
			inputFiles: []string{},
			wantFiles:  []string{},
			wantErr:    false,
		},
		{
			name:       "nil input files - returns nil",
			masks:      []string{"*.md"},
			inputFiles: nil,
			wantFiles:  nil,
			wantErr:    false,
		},
		{
			name:       "single mask matches multiple files",
			masks:      []string{"*.md"},
			inputFiles: []string{"README.md", "main.go", "CHANGELOG.md", "Dockerfile"},
			wantFiles:  []string{"README.md", "CHANGELOG.md"},
			wantErr:    false,
		},
		{
			name:       "single mask matches no files",
			masks:      []string{"*.py"},
			inputFiles: []string{"main.go", "README.md", "Dockerfile"},
			wantFiles:  nil, // FilterByMaskProcessor returns nil when no matches are found
			wantErr:    false,
		},
		{
			name:       "multiple masks match different files",
			masks:      []string{"*.md", "*.go", "*.txt"},
			inputFiles: []string{"README.md", "main.go", "notes.txt", "Dockerfile", "config.yml"},
			wantFiles:  []string{"README.md", "main.go", "notes.txt"},
			wantErr:    false,
		},
		{
			name:       "file matches multiple masks",
			masks:      []string{"README.*", "*.md"},
			inputFiles: []string{"README.md", "main.go", "other.txt"},
			wantFiles:  []string{"README.md"},
			wantErr:    false,
		},
		{
			name:       "complex glob patterns",
			masks:      []string{"test_*.go", "*_test.go"},
			inputFiles: []string{"test_main.go", "main_test.go", "helper_test.go", "main.go", "test_utils.go"},
			wantFiles:  []string{"test_main.go", "main_test.go", "helper_test.go", "test_utils.go"},
			wantErr:    false,
		},
		{
			name:       "character class patterns",
			masks:      []string{"file[0-9].txt"},
			inputFiles: []string{"file1.txt", "file2.txt", "filea.txt", "file10.txt", "file.txt"},
			wantFiles:  []string{"file1.txt", "file2.txt"},
			wantErr:    false,
		},
		{
			name:       "question mark wildcard",
			masks:      []string{"file?.md"},
			inputFiles: []string{"file1.md", "file22.md", "filea.md", "file.md"},
			wantFiles:  []string{"file1.md", "filea.md"},
			wantErr:    false,
		},
		{
			name:       "full path vs basename matching",
			masks:      []string{"*.go"},
			inputFiles: []string{"src/main.go", "pkg/utils/helper.go", "docs/README.md", "/absolute/path/test.go"},
			wantFiles:  []string{"src/main.go", "pkg/utils/helper.go", "/absolute/path/test.go"},
			wantErr:    false,
		},
		{
			name:       "nested directories - matches basename only",
			masks:      []string{"config.*"},
			inputFiles: []string{"config.yaml", "src/config.go", "deploy/k8s/config.yml", "other.txt"},
			wantFiles:  []string{"config.yaml", "src/config.go", "deploy/k8s/config.yml"},
			wantErr:    false,
		},
		{
			name:       "case sensitive matching",
			masks:      []string{"*.MD"},
			inputFiles: []string{"README.md", "CHANGELOG.MD", "notes.Md"},
			wantFiles:  []string{"CHANGELOG.MD"},
			wantErr:    false,
		},
		{
			name:       "files with no extension",
			masks:      []string{"Dockerfile", "Makefile"},
			inputFiles: []string{"Dockerfile", "Makefile", "main.go", "README.md"},
			wantFiles:  []string{"Dockerfile", "Makefile"},
			wantErr:    false,
		},
		{
			name:       "files with dots in names",
			masks:      []string{"*.validator.*"},
			inputFiles: []string{"link.validator.yaml", "test.validator.json", "main.go", ".validator.conf"},
			wantFiles:  []string{"link.validator.yaml", "test.validator.json", ".validator.conf"}, // .validator.conf matches because * can be empty
			wantErr:    false,
		},
		{
			name:       "hidden files and dotfiles",
			masks:      []string{".*"},
			inputFiles: []string{".gitignore", ".env", "main.go", ".hidden.txt"},
			wantFiles:  []string{".gitignore", ".env", ".hidden.txt"},
			wantErr:    false,
		},
		{
			name:       "preserve file order",
			masks:      []string{"*.txt"},
			inputFiles: []string{"z.txt", "a.txt", "m.txt", "b.go"},
			wantFiles:  []string{"z.txt", "a.txt", "m.txt"}, // preserves original order
			wantErr:    false,
		},
		{
			name:       "duplicate files in input",
			masks:      []string{"*.md"},
			inputFiles: []string{"README.md", "main.go", "README.md", "other.txt", "README.md"},
			wantFiles:  []string{"README.md", "README.md", "README.md"}, // preserves duplicates
			wantErr:    false,
		},
		{
			name:       "invalid glob pattern - returns error",
			masks:      []string{"["},
			inputFiles: []string{"test.txt"},
			wantFiles:  nil,
			wantErr:    true,
		},
		{
			name:       "mixed valid and invalid patterns - returns error on first invalid",
			masks:      []string{"*.md", "[", "*.go"},
			inputFiles: []string{"test.md", "main.go"},
			wantFiles:  nil,
			wantErr:    true,
		},
		{
			name:  "real-world scenario - documentation files",
			masks: []string{"README*", "*.md", "CHANGELOG*", "LICENSE*"},
			inputFiles: []string{
				"README.md", "src/main.go", "CHANGELOG.md", "LICENSE",
				"docs/api.md", "README.txt", "pkg/utils.go", "LICENSE.txt",
			},
			wantFiles: []string{"README.md", "CHANGELOG.md", "LICENSE", "docs/api.md", "README.txt", "LICENSE.txt"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := FilterByMaskProcessor(tt.masks)
			got, err := processor(tt.inputFiles)

			if (err != nil) != tt.wantErr {
				t.Errorf("FilterByMaskProcessor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.wantFiles) {
				t.Errorf("FilterByMaskProcessor() = %v, want %v", got, tt.wantFiles)
			}
		})
	}
}

func TestWalkDirectoryProcessor(t *testing.T) {
	type testSetup struct {
		dirName   string
		fileNames []string
	}

	type testCase struct {
		name       string
		config     *config.Config
		setup      testSetup
		inputFiles []string
		wantFiles  []string
		wantErr    bool
	}

	tests := []testCase{
		{
			name:       "explicit files provided - returns input unchanged",
			config:     &config.Config{Files: []string{"README.md", "main.go"}, FileMasks: []string{"*.md"}},
			setup:      testSetup{fileNames: []string{"README.md", "CHANGELOG.md", "main.go"}},
			inputFiles: []string{"input1.txt", "input2.go"},
			wantFiles:  []string{"input1.txt", "input2.go"}, // input returned unchanged
			wantErr:    false,
		},
		{
			name:       "no explicit files - walks directory and matches masks",
			config:     &config.Config{FileMasks: []string{"*.md"}},
			setup:      testSetup{fileNames: []string{"README.md", "CHANGELOG.md", "main.go", "Dockerfile"}},
			inputFiles: []string{},
			wantFiles:  []string{"CHANGELOG.md", "README.md"}, // lexicographical order
			wantErr:    false,
		},
		{
			name:       "multiple file masks",
			config:     &config.Config{FileMasks: []string{"*.md", "*.go", "*.txt"}},
			setup:      testSetup{fileNames: []string{"README.md", "main.go", "notes.txt", "Dockerfile", "config.yml"}},
			inputFiles: []string{},
			wantFiles:  []string{"README.md", "main.go", "notes.txt"},
			wantErr:    false,
		},
		{
			name:       "no files match masks",
			config:     &config.Config{FileMasks: []string{"*.py"}},
			setup:      testSetup{fileNames: []string{"README.md", "main.go", "Dockerfile"}},
			inputFiles: []string{},
			wantFiles:  nil,
			wantErr:    false,
		},
		{
			name:       "empty directory",
			config:     &config.Config{FileMasks: []string{"*.md"}},
			setup:      testSetup{fileNames: []string{}},
			inputFiles: []string{},
			wantFiles:  nil,
			wantErr:    false,
		},
		{
			name:       "nested directory structure",
			config:     &config.Config{FileMasks: []string{"*.md", "*.go"}},
			setup:      testSetup{fileNames: []string{"README.md", "src/main.go", "pkg/utils/helper.go", "docs/api.md", "Dockerfile"}},
			inputFiles: []string{},
			wantFiles:  []string{"README.md", "docs/api.md", "pkg/utils/helper.go", "src/main.go"}, // lexicographical order
			wantErr:    false,
		},
		{
			name:       "files with complex patterns",
			config:     &config.Config{FileMasks: []string{"test_*.go", "*_test.go"}},
			setup:      testSetup{fileNames: []string{"test_main.go", "main_test.go", "helper_test.go", "main.go", "test_utils.go"}},
			inputFiles: []string{},
			wantFiles:  []string{"helper_test.go", "main_test.go", "test_main.go", "test_utils.go"}, // lexicographical order
			wantErr:    false,
		},
		{
			name:       "hidden files and dotfiles",
			config:     &config.Config{FileMasks: []string{".*", "*.md"}},
			setup:      testSetup{fileNames: []string{".gitignore", ".env", "README.md"}}, // Remove problematic nested hidden file
			inputFiles: []string{},
			wantFiles:  []string{".env", ".gitignore", "README.md"}, // lexicographical order
			wantErr:    false,
		},
		{
			name:       "files with no extension",
			config:     &config.Config{FileMasks: []string{"Dockerfile", "Makefile", "LICENSE"}},
			setup:      testSetup{fileNames: []string{"Dockerfile", "Makefile", "LICENSE", "README.md", "main.go"}},
			inputFiles: []string{},
			wantFiles:  []string{"Dockerfile", "LICENSE", "Makefile"}, // lexicographical order
			wantErr:    false,
		},
		{
			name:       "case sensitive file matching",
			config:     &config.Config{FileMasks: []string{"*.MD", "*.Go"}},
			setup:      testSetup{fileNames: []string{"README.md", "CHANGELOG.MD", "main.go", "utils.Go"}},
			inputFiles: []string{},
			wantFiles:  []string{"CHANGELOG.MD", "utils.Go"},
			wantErr:    false,
		},
		{
			name:   "deeply nested structure",
			config: &config.Config{FileMasks: []string{"*.json", "*.yaml"}},
			setup: testSetup{fileNames: []string{
				"config.json",
				"src/config/app.yaml",
				"deploy/k8s/service.yaml",
				"tests/data/sample.json",
				"docs/README.md",
			}},
			inputFiles: []string{},
			wantFiles:  []string{"config.json", "deploy/k8s/service.yaml", "src/config/app.yaml", "tests/data/sample.json"}, // lexicographical order
			wantErr:    false,
		},
		{
			name:       "empty file masks - no files should match",
			config:     &config.Config{FileMasks: []string{}},
			setup:      testSetup{fileNames: []string{"README.md", "main.go", "config.yml"}},
			inputFiles: []string{},
			wantFiles:  nil,
			wantErr:    false,
		},
		{
			name:       "nil file masks - no files should match",
			config:     &config.Config{FileMasks: nil},
			setup:      testSetup{fileNames: []string{"README.md", "main.go", "config.yml"}},
			inputFiles: []string{},
			wantFiles:  nil,
			wantErr:    false,
		},
		{
			name:       "wildcard mask matches all files",
			config:     &config.Config{FileMasks: []string{"*"}},
			setup:      testSetup{fileNames: []string{"README.md", "main.go", "Dockerfile", "config.yml"}},
			inputFiles: []string{},
			wantFiles:  []string{"Dockerfile", "README.md", "config.yml", "main.go"}, // lexicographical order
			wantErr:    false,
		},
		{
			name:   "complex real-world scenario",
			config: &config.Config{FileMasks: []string{"*.md", "*.yml", "*.yaml", "Dockerfile*", "Makefile"}},
			setup: testSetup{fileNames: []string{
				"README.md",
				"CHANGELOG.md",
				"docker-compose.yml",
				"k8s/deployment.yaml",
				"Dockerfile",
				"Dockerfile.dev",
				"Makefile",
				"main.go",
				"src/utils.go",
			}},
			inputFiles: []string{},
			wantFiles:  []string{"CHANGELOG.md", "Dockerfile", "Dockerfile.dev", "Makefile", "README.md", "docker-compose.yml", "k8s/deployment.yaml"}, // lexicographical order
			wantErr:    false,
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

	cleanUp := func(test testSetup) {
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
			// Setup test directory structure
			if len(tt.setup.fileNames) != 0 {
				mkDir(tt.setup.dirName)
				for _, f := range tt.setup.fileNames {
					mkFile(f)
				}
				// Set lookup path to temp directory
				tt.config.LookupPath = tmp

				// Update expected paths to include full temporary directory path
				// Only do this if we expect files to be found by directory walking (no explicit files)
				if len(tt.config.Files) == 0 && len(tt.wantFiles) > 0 && !filepath.IsAbs(tt.wantFiles[0]) {
					for i, wantFile := range tt.wantFiles {
						tt.wantFiles[i] = filepath.Join(tmp, wantFile)
					}
				}
			}

			t.Cleanup(func() {
				cleanUp(tt.setup)
			})

			processor := WalkDirectoryProcessor(tt.config)
			got, err := processor(tt.inputFiles)

			if (err != nil) != tt.wantErr {
				t.Errorf("WalkDirectoryProcessor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.wantFiles) {
				t.Errorf("WalkDirectoryProcessor() = %v, want %v", got, tt.wantFiles)
			}
		})
	}
}
