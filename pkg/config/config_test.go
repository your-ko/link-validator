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
			name: "merge_nil_config_does_nothing",
			fields: fields{
				cfg: &Config{
					PAT:       "existing-pat",
					FileMasks: []string{"*.md"},
					Timeout:   5 * time.Second,
				},
			},
			args: args{
				config: nil,
			},
			want: &Config{
				PAT:       "existing-pat",
				FileMasks: []string{"*.md"},
				Timeout:   5 * time.Second,
			},
		},
		{
			name: "merge_empty_config_does_nothing",
			fields: fields{
				cfg: &Config{
					PAT:       "existing-pat",
					FileMasks: []string{"*.md"},
					Timeout:   5 * time.Second,
				},
			},
			args: args{
				config: &Config{},
			},
			want: &Config{
				PAT:       "existing-pat",
				FileMasks: []string{"*.md"},
				Timeout:   5 * time.Second,
			},
		},
		{
			name: "merge_overwrites_non_empty_values",
			fields: fields{
				cfg: &Config{
					PAT:           "old-pat",
					CorpPAT:       "old-corp-pat",
					CorpGitHubUrl: "old-url",
					FileMasks:     []string{"*.md"},
					Timeout:       3 * time.Second,
				},
			},
			args: args{
				config: &Config{
					PAT:            "new-pat",
					CorpGitHubUrl:  "new-url",
					FileMasks:      []string{"*.txt", "*.go"},
					Timeout:        10 * time.Second,
					IgnoredDomains: []string{"example.com"},
				},
			},
			want: &Config{
				PAT:            "new-pat",
				CorpPAT:        "old-corp-pat", // Not overwritten because merge config has empty value
				CorpGitHubUrl:  "new-url",
				FileMasks:      []string{"*.txt", "*.go"},
				Timeout:        10 * time.Second,
				IgnoredDomains: []string{"example.com"},
			},
		},
		{
			name: "merge_preserves_existing_when_merge_config_has_empty_values",
			fields: fields{
				cfg: &Config{
					PAT:            "existing-pat",
					CorpPAT:        "existing-corp-pat",
					CorpGitHubUrl:  "existing-url",
					FileMasks:      []string{"*.md"},
					LookupPath:     "existing-path",
					Timeout:        5 * time.Second,
					IgnoredDomains: []string{"existing.com"},
				},
			},
			args: args{
				config: &Config{
					PAT: "new-pat", // Only this gets merged
				},
			},
			want: &Config{
				PAT:            "new-pat",
				CorpPAT:        "existing-corp-pat",
				CorpGitHubUrl:  "existing-url",
				FileMasks:      []string{"*.md"},
				LookupPath:     "existing-path",
				Timeout:        5 * time.Second,
				IgnoredDomains: []string{"existing.com"},
			},
		},
		{
			name: "merge_handles_zero_timeout_correctly",
			fields: fields{
				cfg: &Config{
					Timeout: 5 * time.Second,
				},
			},
			args: args{
				config: &Config{
					PAT:     "new-pat",
					Timeout: 0, // Zero timeout should not override
				},
			},
			want: &Config{
				PAT:     "new-pat",
				Timeout: 5 * time.Second, // Should remain unchanged
			},
		},
		{
			name: "merge_empty_slices_do_not_override",
			fields: fields{
				cfg: &Config{
					FileMasks:      []string{"*.md", "*.txt"},
					IgnoredDomains: []string{"example.com"},
				},
			},
			args: args{
				config: &Config{
					PAT:            "new-pat",
					FileMasks:      []string{}, // Empty slice should not override
					IgnoredDomains: nil,        // nil slice should not override
				},
			},
			want: &Config{
				PAT:            "new-pat",
				FileMasks:      []string{"*.md", "*.txt"}, // Should remain unchanged
				IgnoredDomains: []string{"example.com"},   // Should remain unchanged
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fields.cfg.merge(tt.args.config)
			if !reflect.DeepEqual(tt.fields.cfg, tt.want) {
				t.Errorf("merge() got = %v, want %v", tt.fields.cfg, tt.want)
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
			name: "empty_config_returns_nil",
			fields: fields{
				config: "",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "whitespace_only_config_returns_nil",
			fields: fields{
				config: "   \n\n  ",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "valid_yaml_config_parsed_successfully",
			fields: fields{
				config: `fileMasks:
  - "*.md"
  - "*.txt"
timeout: 5s
corpGitHubUrl: "https://github.mycorp.com"
ignoredDomains:
  - "example.com"
  - "test.org"`,
			},
			want: &Config{
				FileMasks:      []string{"*.md", "*.txt"},
				Timeout:        5 * time.Second,
				CorpGitHubUrl:  "https://github.mycorp.com",
				IgnoredDomains: []string{"example.com", "test.org"},
			},
			wantErr: false,
		},
		{
			name: "partial_config_loads_only_specified_fields",
			fields: fields{
				config: `timeout: 10s
fileMasks:
  - "*.go"`,
			},
			want: &Config{
				Timeout:   10 * time.Second,
				FileMasks: []string{"*.go"},
			},
			wantErr: false,
		},
		{
			name: "malformed_yaml_returns_error",
			fields: fields{
				config: `fileMasks:
  - "*.md"
timeout: invalid_yaml: {`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "unknown_field_returns_error",
			fields: fields{
				config: `fileMasks:
  - "*.md"
timeout: 3s
unknownField: "should cause error"`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid_duration_format_returns_error",
			fields: fields{
				config: `timeout: "not-a-duration"`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "yaml_with_comments_parses_successfully",
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
			cfg := Default().WithReader(r)
			got, err := cfg.loadFromReader()
			if (err != nil) != tt.wantErr {
				t.Errorf("loadFromReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadFromReader() got = %v, want %v", got, tt.want)
			}
		})
	}
}
