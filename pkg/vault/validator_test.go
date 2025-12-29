package vault

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/hashicorp/vault-client-go"
	"github.com/stretchr/testify/mock"
)

var gotVaultErr *vault.ResponseError

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

func Test_validateSecret(t *testing.T) {
	type args struct {
		secretPath string
	}
	tests := []struct {
		name      string
		args      args
		setupMock func(*mockvaultClient)
		wantErr   error
	}{
		{
			name: "secret exists",
			args: args{"/secret/general/big-secret"},
			setupMock: func(m *mockvaultClient) {
				listNotFoundErr := &vault.ResponseError{
					StatusCode: http.StatusNotFound,
				}
				resp := &vault.Response[map[string]interface{}]{
					Data: map[string]interface{}{
						"keys": []string{"some-secret"},
					},
				}
				m.EXPECT().List(mock.Anything, "/secret/general/big-secret").Return(nil, listNotFoundErr)
				m.EXPECT().Read(mock.Anything, "/secret/general/big-secret").Return(resp, nil)
			},
		},
		{
			name: "secret doesn't exist",
			args: args{"/secret/general/big-secret"},
			setupMock: func(m *mockvaultClient) {
				NotFoundErr := &vault.ResponseError{
					StatusCode: http.StatusNotFound,
				}
				m.EXPECT().List(mock.Anything, "/secret/general/big-secret").Return(nil, NotFoundErr)
				m.EXPECT().Read(mock.Anything, "/secret/general/big-secret").Return(nil, NotFoundErr)
			},
			wantErr: &vault.ResponseError{
				StatusCode: http.StatusNotFound,
			},
		},
		{
			name: "secret folder exists",
			args: args{"/secret/general/"},
			setupMock: func(m *mockvaultClient) {
				resp := &vault.Response[map[string]interface{}]{
					Data: map[string]interface{}{
						"keys": []string{"some-secret"},
					},
				}
				m.EXPECT().List(mock.Anything, "/secret/general/").Return(resp, nil)
			},
		},
		{
			name: "secret folder not found",
			args: args{"/secret/general/"},
			setupMock: func(m *mockvaultClient) {
				NotFoundErr := &vault.ResponseError{
					StatusCode: http.StatusNotFound,
				}
				m.EXPECT().List(mock.Anything, "/secret/general/").Return(nil, NotFoundErr)
				m.EXPECT().Read(mock.Anything, "/secret/general/").Return(nil, NotFoundErr)
			},
			wantErr: &vault.ResponseError{
				StatusCode: http.StatusNotFound,
			},
		},
		{
			name: "requested no particular path, so just connectivity check",
			args: args{"/"},
			setupMock: func(m *mockvaultClient) {
				forbidderErr := &vault.ResponseError{
					StatusCode: http.StatusForbidden,
				}
				m.EXPECT().List(mock.Anything, "/").Return(nil, forbidderErr)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockvaultClient(t)
			tt.setupMock(mockClient)

			err := validateSecret(context.Background(), mockClient, tt.args.secretPath)

			if !mockClient.AssertExpectations(t) {
				return
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}

			if tt.wantErr.Error() != err.Error() {
				t.Fatalf("expected error message:\n%q\ngot:\n%q", tt.wantErr.Error(), err.Error())
			}

			if errors.As(tt.wantErr, &gotVaultErr) && !errors.As(err, &gotVaultErr) {
				t.Fatalf("expected error to be *github.ErrorResponse, got %T", err)
			}
		})
	}
}
