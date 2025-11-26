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
