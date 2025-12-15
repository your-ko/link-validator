package dd

import (
	"context"
	"fmt"
	"link-validator/pkg/config"
	"link-validator/pkg/regex"
	"log/slog"
	"net/url"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

type LinkProcessor struct {
	client *wrapper
	routes map[string]ddHandler
}

type ddResource struct {
	Type      string
	ID        string
	Action    string
	Query     url.Values
	Fragments string
	Path      []string
}

func New(cfg *config.Config) (*LinkProcessor, error) {
	if cfg.DDApiKey == "" || cfg.DDAppKey == "" {
		return nil, fmt.Errorf("can't initialise DataDog client, DD_API_KEY/DD_APP_KEY are not set")
	}
	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)

	proc := &LinkProcessor{
		client: &wrapper{
			client: apiClient,
			apiKey: cfg.DDApiKey,
			appKey: cfg.DDAppKey,
		},
		routes: make(map[string]ddHandler),
	}
	return proc.registerDefaultHandlers(), nil
}

func (proc *LinkProcessor) registerDefaultHandlers() *LinkProcessor {
	return proc.
		Route("", handleConnection).
		Route("monitors", handleMonitors).
		Route("dashboard", handleDashboards)
	//Route("logs", proc.validateConnection).
	//Route("apm", proc.validateConnection).
	//Route("infrastructure", proc.validateConnection).
	//Route("synthetics", proc.validateConnection).
	//Route("account", proc.validateConnection).
	//Route("organization-settings", proc.validateConnection)
}

func (proc *LinkProcessor) Process(ctx context.Context, link string, _ string) error {
	slog.Debug("Validating DataDog URL", slog.String("url", link))

	// Parse URL
	resource, err := parseDataDogURL(link)
	if err != nil {
		return err
	}

	if handler, exists := proc.routes[resource.Type]; exists {
		return handler(ctx, proc.client, *resource)
	}
	return fmt.Errorf("unsupported DataDog URL type: '%s'", resource.Type)
}

func parseDataDogURL(link string) (*ddResource, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	pathSegments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathSegments) == 1 && pathSegments[0] == "" {
		pathSegments = []string{} // Empty path
	}

	resource := &ddResource{
		Path:      pathSegments,
		Query:     u.Query(),
		Fragments: u.Fragment,
	}
	if len(pathSegments) == 0 {
		return resource, nil
	}
	growTo := 5 // some arbitrary number more than 5 to prevent 'len out of range' during parsing below
	if len(pathSegments) < growTo {
		diff := growTo - len(pathSegments)
		pathSegments = append(pathSegments, make([]string, diff)...)[:growTo]
	}

	resource.Type = pathSegments[0]
	switch resource.Type {
	case "monitors":
		resource.ID = pathSegments[1]
		resource.Action = pathSegments[2]
	case "dashboards":
		resource.ID = pathSegments[1]
		resource.Action = pathSegments[2]
	}

	return resource, nil
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

func (proc *LinkProcessor) Route(resourceType string, handler ddHandler) *LinkProcessor {
	proc.routes[resourceType] = handler
	return proc
}
