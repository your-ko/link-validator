package vault

import (
	"context"

	"github.com/hashicorp/vault-client-go"
)

type vaultClient interface {
	List(ctx context.Context, path string, options ...vault.RequestOption) (*vault.Response[map[string]interface{}], error)
	Read(ctx context.Context, path string, options ...vault.RequestOption) (*vault.Response[map[string]interface{}], error)
}

type wrapper struct {
	client *vault.Client
}

func (w wrapper) List(ctx context.Context, path string, options ...vault.RequestOption) (*vault.Response[map[string]interface{}], error) {
	return w.client.List(ctx, path, options...)
}

func (w wrapper) Read(ctx context.Context, path string, options ...vault.RequestOption) (*vault.Response[map[string]interface{}], error) {
	return w.client.Read(ctx, path, options...)
}
