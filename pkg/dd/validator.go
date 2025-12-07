package dd

import (
	"context"
	"fmt"
	"link-validator/pkg/config"
	"link-validator/pkg/regex"
	"net/url"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

type LinkProcessor struct {
	client *datadog.APIClient
}

func New(cfg *config.Config) (*LinkProcessor, error) {
	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)

	ctx := datadog.NewDefaultContext(context.Background())
	if cfg.DDApiKey == "" || cfg.DDAppKey == "" {
		return nil, fmt.Errorf("can't initialise DataDog client, DD_API_KEY/DD_APP_KEY are not set")
	}
	ctx = context.WithValue(ctx, datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {
			Key: cfg.DDApiKey,
		},
		"appKeyAuth": {
			Key: cfg.DDAppKey,
		},
	})
	return &LinkProcessor{
		client: apiClient,
	}, nil
}

func (proc *LinkProcessor) Process(ctx context.Context, url string, testFileName string) error {
	//TODO implement me
	panic("implement me")
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	parts := regex.DataDog.FindAllString(line, -1)
	if len(parts) == 0 {
		return nil
	}

	urls := make([]string, 0, len(parts))
	for _, raw := range parts {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue // skip malformed
		}
		if strings.ContainsAny(raw, "[]{}()") {
			continue // seems it is the templated url
		}

		urls = append(urls, raw)
	}

	return urls
}
