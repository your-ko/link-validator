package local

import (
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
