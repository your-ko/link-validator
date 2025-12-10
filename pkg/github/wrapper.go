package github

import (
	"context"

	"github.com/google/go-github/v77/github"
)

type Client interface {
	Repositories(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
}

type wrapper struct {
	client *github.Client
}

func (w *wrapper) Repositories(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
	return w.client.Repositories.Get(ctx, owner, repo)
}
