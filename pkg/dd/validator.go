package dd

import (
	"context"
	"fmt"
	"link-validator/pkg/config"
	"link-validator/pkg/regex"
	"log/slog"
	"net/url"
	"path"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

type LinkProcessor struct {
	client *wrapper
	routes map[string]ddHandler
}

type ddResource struct {
	typ      string
	id       string
	subType  string
	action   string
	query    url.Values
	fragment string
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
		Route("ddsql", handleConnection).
		Route("sheets", handleConnection).           // currently there is no API to fetch sheets
		Route("dash/integration", handleConnection). // dashboards coming from integrations are not accessible via API
		Route("monitors", handleMonitors).
		Route("dashboard", handleDashboards).
		Route("notebook", handleNotebooks)

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

	if handler, exists := proc.routes[resource.typ]; exists {
		return handler(ctx, proc.client, *resource)
	}
	return fmt.Errorf("unsupported DataDog URL type: '%s'", resource.typ)
}

func parseDataDogURL(link string) (*ddResource, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		segments = []string{} // Empty path
	}

	resource := &ddResource{
		query:    u.Query(),
		fragment: u.Fragment,
	}
	if len(segments) == 0 {
		return resource, nil
	}
	growTo := 5 // some arbitrary number more than 5 to prevent 'len out of range' during parsing below
	if len(segments) < growTo {
		diff := growTo - len(segments)
		segments = append(segments, make([]string, diff)...)[:growTo]
	}

	resource.typ = segments[0]
	switch resource.typ {
	case "monitors":
		if isEmpty(segments[1:]) {
			resource.typ = ""
		} else {
			switch segments[1] {
			case "manage", "edit":
				resource.action = segments[1]
			default:
				resource.id = segments[1]
				resource.action = segments[2]
			}
		}
	case "dashboard":
		switch segments[1] {
		case "lists", "reports", "shared":
			if isEmpty(segments[2:]) {
				resource.typ = ""
			} else {
				resource.subType = path.Join(segments[1], segments[2])
				resource.id = segments[3]
			}
			if resource.subType == "lists/preset" {
				// currently there is no DD API to fetch preset lists of dashboards
				resource.typ = ""
			}
		default:
			resource.id = segments[1]
			resource.subType = segments[2]
		}
	case "dash":
		resource.typ = path.Join(resource.typ, segments[1])
		resource.id = segments[2]
	case "ddsql":
		resource.subType = segments[1]
	case "notebook":
		if segments[1] == "reports" || segments[1] == "template-gallery" || segments[1] == "list" {
			resource.subType = segments[1]
			resource.typ = ""
		} else if segments[1] == "custom-template" {
			resource.id = segments[2]
			resource.subType = segments[1]
		} else {
			resource.id = segments[1]
		}
	case "sheets":
		resource.id = segments[1]
	}

	return resource, nil
}

func isEmpty(segments []string) bool {
	for _, s := range segments {
		if s != "" {
			return false
		}
	}
	return true
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
