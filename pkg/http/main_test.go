package http

import (
	"reflect"
	"testing"
)

func TestExternalHttpLinkProcessor_ExtractLinks(t *testing.T) {
	t.Parallel()

	type tc struct {
		name    string
		exclude string
		line    string
		want    []string
	}

	tests := []tc{
		{
			name:    "exclude exact host and its subdomains",
			exclude: "https://github.mycorp.com",
			line: `see https://github.mycorp.com/org/repo
			       and https://api.github.mycorp.com/x
			       and https://example.com/page
			       and https://gitlab.mycorp.com/y`,
			// Expect to keep only non-excluded domains.
			want: []string{
				"https://example.com/page",
				"https://gitlab.mycorp.com/y",
			},
		},
		{
			name:    "exclude without scheme still filters",
			exclude: "github.mycorp.com",
			line:    `https://github.mycorp.com a https://api.github.mycorp.com b https://google.com?q=1`,
			want: []string{
				"https://google.com?q=1",
			},
		},
		{
			name:    "exclude with leading dot works same",
			exclude: ".github.mycorp.com",
			line:    `https://github.mycorp.com https://sub.github.mycorp.com https://other.com`,
			want: []string{
				"https://other.com",
			},
		},
		{
			name:    "http links are not matched by regex (https only)",
			exclude: "github.mycorp.com",
			line:    `http://github.mycorp.com https://github.mycorp.com https://ok.com`,
			want: []string{
				// http://... is ignored by regex; https://github.mycorp.com excluded; only ok.com remains
				"https://ok.com",
			},
		},
		{
			name:    "mixed unrelated https remain",
			exclude: "github.mycorp.com",
			line:    `https://one.com https://two.com https://github.mycorp.com https://three.com`,
			want: []string{
				"https://one.com",
				"https://two.com",
				"https://three.com",
			},
		},
		{
			name:    "test for MD link",
			exclude: "github.mycorp.com",
			line:    `qqq https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/main.yaml) qqq`,
			want: []string{
				"https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg",
				"https://github.com/your-ko/link-validator/actions/workflows/main.yaml",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := New(tt.exclude)
			got := proc.ExtractLinks(tt.line)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks() mismatch\nexclude=%q\nline=%q\ngot = %#v\nwant= %#v",
					tt.exclude, tt.line, got, tt.want)
			}
		})
	}
}
