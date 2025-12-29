package vault

import "testing"

func Test_transformPath(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "nested path with show",
			args: args{path: "/ui/vault/secrets/kv/show/app/database/credentials"},
			want: "/kv/app/database/credentials",
		},
		{
			name: "nested path without show",
			args: args{path: "/ui/vault/secrets/kv/app/database/credentials"},
			want: "/kv/app/database/credentials",
		},
		{
			name: "path without prefix",
			args: args{path: "kv/show/secret1"},
			want: "/",
		},
		{
			name: "path with partial prefix",
			args: args{path: "/ui/vault/kv/show/secret1"},
			want: "/",
		},
		{
			name: "show as second segment no trailing",
			args: args{path: "/ui/vault/secrets/kv/show"},
			want: "/kv",
		},
		{
			name: "multiple show segments",
			args: args{path: "/ui/vault/secrets/kv/show/show/secret1"},
			want: "/kv/show/secret1",
		},
		{
			name: "secrets list",
			args: args{path: "/ui/vault/secrets/kv/list/folder/"},
			want: "/kv/folder/",
		},
		{
			name: "path with spaces",
			args: args{path: "/ui/vault/secrets/kv/show/secret with spaces"},
			want: "/kv/secret with spaces",
		},
		{
			name: "path with special_chars",
			args: args{path: "/ui/vault/secrets/kv/show/secret-name_123"},
			want: "/kv/secret-name_123",
		},
		{
			name: "single slash",
			args: args{path: "/"},
			want: "/",
		},
		{
			name: "empty path",
			args: args{path: ""},
			want: "/",
		},
		{
			name: "UI root",
			args: args{path: "/ui/vault/secrets"},
			want: "/",
		},
		{
			name: "dashboard",
			args: args{path: "/ui/vault/dashboard"},
			want: "/",
		},
		{
			name: "entities",
			args: args{path: "/ui/vault/access/identity/entities"},
			want: "/",
		},
		{
			name: "no leading slash", // TODO: think
			args: args{path: "ui/vault/secrets/kv/show/secret1"},
			want: "/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transformPath(tt.args.path); got != tt.want {
				t.Errorf("transformPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
