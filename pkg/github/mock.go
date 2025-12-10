package github

import (
	"context"

	"github.com/google/go-github/v77/github"
	"github.com/stretchr/testify/mock"
)

type MockClient struct {
	mock.Mock
}

func (m MockClient) Repositories(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
	args := m.Called(ctx, owner, repo)
	return args.Get(0).(*github.Repository), args.Get(1).(*github.Response), args.Error(2)
}
