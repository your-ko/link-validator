package local

import (
	"reflect"
	"testing"
)

func TestLinkProcessor_ExtractLinks_LocalOnly(t *testing.T) {
	t.Parallel()

	proc := New("")

	type tc struct {
		name string
		line string
		want []string // full markdown link matches like "[txt](../../README.md)"
	}

	tests := []tc{
		{
			name: "single relative up-two-levels",
			line: `see [readme](../../README.md) for details`,
			want: []string{
				"[readme](../../README.md)",
			},
		},
		{
			name: "relative current dir and parent dir",
			line: `open [doc](./docs/guide.md) and [up](../CONTRIBUTING.md)`,
			want: []string{
				"[doc](./docs/guide.md)",
				"[up](../CONTRIBUTING.md)",
			},
		},
		{
			name: "bare filename is local",
			line: `see [root](README.md)`,
			want: []string{
				"[root](README.md)",
			},
		},
		{
			name: "mixed local and external – keep only local",
			line: `local [a](./a.md), external [g](https://google.com), local [b](../b.md)`,
			want: []string{
				"[a](./a.md)",
				"[b](../b.md)",
			},
		},
		{
			name: "ignore mailto and http",
			line: `email [me](mailto:me@example.com) and site [ex](http://example.com) and local [c](docs/c.md)`,
			want: []string{
				"[c](docs/c.md)",
			},
		},
		{
			name: "ignore anchors",
			line: `section [here](#intro) and local [spec](specs/api.md)`,
			want: []string{
				"[spec](specs/api.md)",
			},
		},
		{
			name: "multiple locals on one line",
			line: `[one](a.md) [two](b/c.md) [three](../d/e.md)`,
			want: []string{
				"[one](a.md)",
				"[two](b/c.md)",
				"[three](../d/e.md)",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proc.ExtractLinks(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks mismatch\nline=%q\ngot = %#v\nwant= %#v",
					tt.line, got, tt.want)
			}
		})
	}
}
