package git

import (
	"reflect"
	"testing"
)

func TestInternalLinkProcessor_ExtractLinks(t *testing.T) {
	t.Parallel()

	p := New("https://github.mycorp.com", "") // PAT not needed for regex tests

	type tc struct {
		name string
		line string
		want []string
	}

	tests := []tc{
		{
			name: "keeps internal blob/tree/raw; drops externals",
			line: `see https://github.mycorp.com/org/repo/blob/main/README.md
			       and https://google.com/x
			       then https://github.mycorp.com/org/repo/tree/main/dir
			       and https://example.com/y
			       and https://github.mycorp.com/org/repo/raw/main/file.txt`,
			want: []string{
				"https://github.mycorp.com/org/repo/blob/main/README.md",
				"https://github.mycorp.com/org/repo/tree/main/dir",
				"https://github.mycorp.com/org/repo/raw/main/file.txt",
			},
		},
		{
			name: "includes subdomain uploads.* as internal",
			line: `assets at https://uploads.github.mycorp.com/org/repo/raw/main/image.png
			       and external https://gitlab.mycorp.com/a/b
			       and internal https://github.mycorp.com/acme/proj/blob/main/notes.md`,
			want: []string{
				"https://uploads.github.mycorp.com/org/repo/raw/main/image.png",
				"https://github.mycorp.com/acme/proj/blob/main/notes.md",
			},
		},
		{
			name: "ignores non-matching schemes and hosts",
			line: `http://github.mycorp.com/org/repo/blob/main/README.md
			       https://other.com/org/repo/blob/main/README.md
			       https://api.github.mycorp.com/org/repo/tree/main/folder`,
			want: []string{
				"https://api.github.mycorp.com/org/repo/tree/main/folder",
			},
		},
		{
			name: "handles anchors and query strings",
			line: `https://github.mycorp.com/team/proj/blob/main/file.md#L10-L20
			       https://github.mycorp.com/team/proj/tree/main/docs?tab=readme
			       https://example.com/u/v/raw/main/w.txt?download=1`,
			want: []string{
				"https://github.mycorp.com/team/proj/blob/main/file.md#L10-L20",
				"https://github.mycorp.com/team/proj/tree/main/docs?tab=readme",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := p.ExtractLinks(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks mismatch\nbase=%q\nline=%q\ngot = %#v\nwant= %#v",
					p.baseUrl, tt.line, got, tt.want)
			}
		})
	}
}
