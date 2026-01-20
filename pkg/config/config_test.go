package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConfig_merge(t *testing.T) {
	type fields struct {
		cfg *Config
	}
	type args struct {
		config *Config
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Config
	}{
		{
			name: "merge nil config does nothing",
			fields: fields{
				cfg: &Config{
					Validators: ValidatorsConfig{
						GitHub: GitHubConfig{
							Enabled: true,
							PAT:     "PAT",
						},
					},
					FileMasks: []string{"*.md"},
					Timeout:   5 * time.Second,
				},
			},
			args: args{
				config: nil,
			},
			want: &Config{
				Validators: ValidatorsConfig{
					GitHub: GitHubConfig{
						Enabled: true,
						PAT:     "PAT",
					},
				},
				FileMasks: []string{"*.md"},
				Timeout:   5 * time.Second,
			},
		},
		{
			name: "merge empty config does nothing",
			fields: fields{
				cfg: &Config{
					Validators: ValidatorsConfig{
						GitHub: GitHubConfig{
							Enabled: true,
							PAT:     "PAT",
						},
					},
					FileMasks: []string{"*.md"},
					Timeout:   5 * time.Second,
				},
			},
			args: args{
				config: &Config{},
			},
			want: &Config{
				Validators: ValidatorsConfig{
					GitHub: GitHubConfig{
						Enabled: true,
						PAT:     "PAT",
					},
				},
				FileMasks: []string{"*.md"},
				Timeout:   5 * time.Second,
			},
		},
		{
			name: "merge overwrites non empty values",
			fields: fields{
				cfg: &Config{
					Validators: ValidatorsConfig{
						GitHub: GitHubConfig{
							Enabled:       true,
							PAT:           "OLD_PAT",
							CorpPAT:       "OLD_PAT",
							CorpGitHubUrl: "OLD_URL",
						},
					},
					FileMasks: []string{"*.md"},
					Timeout:   3 * time.Second,
				},
			},
			args: args{
				config: &Config{
					Validators: ValidatorsConfig{
						GitHub: GitHubConfig{
							Enabled:       true,
							PAT:           "NEW_PAT",
							CorpGitHubUrl: "NEW_URL",
						},
						HTTP: HttpConfig{
							Enabled: true,
							Ignore:  []string{"example.com"},
						},
					},
					Timeout: 10 * time.Second,
				},
			},
			want: &Config{
				Validators: ValidatorsConfig{
					GitHub: GitHubConfig{
						Enabled:       true,
						PAT:           "NEW_PAT",
						CorpPAT:       "OLD_PAT",
						CorpGitHubUrl: "NEW_URL",
					},
					HTTP: HttpConfig{
						Enabled: true,
						Ignore:  []string{"example.com"},
					},
				},
				FileMasks: []string{"*.md"},
				Timeout:   10 * time.Second,
			},
		},
		{
			name: "merge preserves existing when merged has empty values",
			fields: fields{
				cfg: &Config{
					Validators: ValidatorsConfig{
						GitHub: GitHubConfig{
							Enabled:       true,
							PAT:           "OLD_PAT",
							CorpPAT:       "OLD_PAT",
							CorpGitHubUrl: "OLD_URL",
						},
						HTTP: HttpConfig{
							Enabled: true,
							Ignore:  []string{"example.com"},
						},
					},
					FileMasks:  []string{"*.md"},
					LookupPath: "existing-path",
					Timeout:    5 * time.Second,
				},
			},
			args: args{
				config: &Config{
					Validators: ValidatorsConfig{
						GitHub: GitHubConfig{
							Enabled: true,
							PAT:     "NEW_PAT",
						},
					},
				},
			},
			want: &Config{
				Validators: ValidatorsConfig{
					GitHub: GitHubConfig{
						Enabled:       true,
						PAT:           "NEW_PAT",
						CorpPAT:       "OLD_PAT",
						CorpGitHubUrl: "OLD_URL",
					},
					HTTP: HttpConfig{
						Enabled: true,
						Ignore:  []string{"example.com"},
					},
				},
				FileMasks:  []string{"*.md"},
				LookupPath: "existing-path",
				Timeout:    5 * time.Second,
			},
		},
		{
			name: "merge handles zero timeout correctly",
			fields: fields{
				cfg: &Config{
					Timeout: 5 * time.Second,
				},
			},
			args: args{
				config: &Config{
					Validators: ValidatorsConfig{
						GitHub: GitHubConfig{
							Enabled: true,
							PAT:     "NEW_PAT",
						},
					},
					Timeout: 0, // Zero timeout should not override
				},
			},
			want: &Config{
				Validators: ValidatorsConfig{
					GitHub: GitHubConfig{
						Enabled: true,
						PAT:     "NEW_PAT",
					},
				},
				Timeout: 5 * time.Second, // Should remain unchanged
			},
		},
		{
			name: "merge slices",
			fields: fields{
				cfg: &Config{
					Validators: ValidatorsConfig{
						HTTP: HttpConfig{
							Enabled: true,
							Ignore:  []string{"example.com"},
						},
					},
					FileMasks: []string{"*.md", "*.txt"},
				},
			},
			args: args{
				config: &Config{
					FileMasks: []string{}, // Empty slice should not override
					Validators: ValidatorsConfig{
						HTTP: HttpConfig{
							Enabled: true,
							Ignore:  nil,
						},
					},
				},
			},
			want: &Config{
				FileMasks: []string{"*.md", "*.txt"}, // Should remain unchanged
				Validators: ValidatorsConfig{
					HTTP: HttpConfig{
						Enabled: true,
						Ignore:  []string{"example.com"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fields.cfg.merge(tt.args.config)
			if !reflect.DeepEqual(tt.fields.cfg, tt.want) {
				t.Errorf("merge() \n"+
					" got: %+v \n"+
					"want: %+v", tt.fields.cfg, tt.want)
			}

		})
	}
}

func TestConfig_loadFromReader(t *testing.T) {
	type fields struct {
		config string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Config
		wantErr bool
	}{
		{
			name: "empty config returns nil",
			fields: fields{
				config: "",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "whitespace only config returns nil",
			fields: fields{
				config: "   \n\n  ",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "valid yaml config",
			fields: fields{
				config: `fileMasks:
 - "*.md"
 - "*.txt"
timeout: 5s
validators:
  github:
    corpUrl: "https://github.mycorp.com"
  http:
    ignore:
     - "example.com"
     - "test.org"`,
			},
			want: &Config{
				FileMasks: []string{"*.md", "*.txt"},
				Timeout:   5 * time.Second,
				Validators: ValidatorsConfig{
					GitHub: GitHubConfig{
						CorpGitHubUrl: "https://github.mycorp.com",
					},
					HTTP: HttpConfig{
						Ignore: []string{"example.com", "test.org"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "partial config loads only specified fields",
			fields: fields{
				config: `timeout: 10s
fileMasks:
 - "*.go"`,
			},
			want: &Config{
				Validators: ValidatorsConfig{
					GitHub:    GitHubConfig{Enabled: false},
					DataDog:   DataDogConfig{Enabled: false},
					LocalPath: ValidatorConfig{Enabled: false},
					HTTP:      HttpConfig{Enabled: false},
				},
				Timeout:   10 * time.Second,
				FileMasks: []string{"*.go"},
			},
			wantErr: false,
		},
		{
			name: "malformed yaml returns error",
			fields: fields{
				config: `fileMasks:
 - "*.md"
timeout: invalid_yaml: {`,
			},
			wantErr: true,
		},
		{
			name: "unknown field returns error",
			fields: fields{
				config: `fileMasks:
 - "*.md"
timeout: 3s
unknownField: "should cause error"`,
			},
			wantErr: true,
		},
		{
			name: "invalid duration format returns error",
			fields: fields{
				config: `timeout: "not-a-duration"`,
			},
			wantErr: true,
		},
		{
			name: "yaml with comments parses successfully",
			fields: fields{
				config: `# Configuration file
fileMasks:  # File patterns to match
 - "*.md"
 - "*.rst"
# Timeout for requests
timeout: 30s`,
			},
			want: &Config{
				FileMasks: []string{"*.md", "*.rst"},
				Timeout:   30 * time.Second,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.fields.config)
			got, err := loadFromReader(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadFromReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadFromReader() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}
