package dd

import (
	"context"
	"encoding/json"
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
		Route("sheets", handleConnection).   // currently there is no API to fetch sheets
		Route("monitors", handleConnection). // generic monitors list or settings
		Route("dash", handleConnection).     // dashboards coming from integrations are not accessible via API
		Route("check", handleConnection).    // not accessible via API
		Route("event", handleConnection).    // events API are not very clear, don't provide a way to decode event ID and I won't want to perform a magic trying to guess it
		Route("monitor", handleMonitors).
		Route("dashboard", handleDashboards).
		Route("notebook", handleNotebooks).
		Route("slo", handleSLO).
		Route("incidents", handleConnection) // Unstable operation 'v2.GetIncident' is disabled
}

func (proc *LinkProcessor) Process(ctx context.Context, link string, _ string) error {
	slog.Debug("datadog: starting validation", slog.String("url", link))

	// Parse URL
	resource, err := parseDataDogURL(link)
	if err != nil {
		return err
	}

	if handler, exists := proc.routes[resource.typ]; exists {
		return mapDDError(link, handler(ctx, proc.client, *resource))
	}
	return fmt.Errorf("unsupported DataDog URL type: '%s'", resource.typ)
}

func parseDataDogURL(link string) (*ddResource, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	segments := prepareSegments(u.Path)
	resource := &ddResource{
		query:    u.Query(),
		fragment: u.Fragment,
	}

	if len(segments) == 0 {
		return resource, nil
	}

	return parseResourceFromSegments(resource, segments), nil
}

func prepareSegments(path string) []string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		return []string{} // Empty path
	}

	// Ensure I have enough segments to prevent index out of range
	const minSegments = 5
	if len(segments) < minSegments {
		padding := make([]string, minSegments-len(segments))
		segments = append(segments, padding...)
	}

	return segments
}

func parseResourceFromSegments(resource *ddResource, segments []string) *ddResource {
	resource.typ = segments[0]

	switch resource.typ {
	case "monitors":
		parseMonitorsResource(resource, segments)
	case "dashboard":
		parseDashboardResource(resource, segments)
	case "dash", "ddsql", "check", "event":
		parseDefaultResource(resource, segments)
	case "notebook":
		parseNotebookResource(resource, segments)
	case "sheets":
		parseSheetsResource(resource, segments)
	case "slo":
		parseSLO(resource, segments)
	case "incidents":
		parseIncidents(resource, segments)
	default:
		resource.typ = "" // generic exit point for not supported DD resources, so just test connection
	}

	return resource
}

func parseSLO(resource *ddResource, segments []string) {
	resource.action = segments[1]
	if len(resource.query) != 0 && resource.query.Has("sp") {
		var sps []sloSPElement
		if err := json.Unmarshal([]byte(resource.query.Get("sp")), &sps); err != nil {
			slog.With("error", err).Error("datadog: can't parse SLO query string, leave SLO id empty")
			return
		}
		resource.id = sps[0].P.ID
		resource.query = url.Values{}
	}
}

func parseMonitorsResource(resource *ddResource, segments []string) {
	if isEmpty(segments[1:]) {
		resource.typ = "monitors"
		return
	}

	resource.typ = "monitor"
	switch segments[1] {
	case "settings":
		resource.typ = "monitors"
		resource.subType = segments[1]
		resource.action = segments[2]
	case "manage", "edit":
		resource.action = segments[1]
	default:
		resource.id = segments[1]
		resource.action = segments[2]
	}
}

func parseDashboardResource(resource *ddResource, segments []string) {
	switch segments[1] {
	case "lists", "reports", "shared":
		if isEmpty(segments[2:]) {
			resource.typ = ""
			return
		}

		resource.subType = path.Join(segments[1], segments[2])
		resource.id = segments[3]

		// Special case: preset lists aren't accessible via API
		if resource.subType == "lists/preset" {
			resource.typ = ""
		}
	default:
		resource.id = segments[1]
		resource.subType = segments[2]
	}
}

func parseDefaultResource(resource *ddResource, segments []string) {
	resource.subType = segments[1]
	resource.id = segments[2]
	resource.query = url.Values{}
}

func parseNotebookResource(resource *ddResource, segments []string) {
	switch segments[1] {
	case "reports", "template-gallery", "list":
		resource.subType = segments[1]
		resource.typ = ""
		return
	case "custom-template":
		resource.id = segments[2]
		resource.subType = segments[1]
	default:
		resource.id = segments[1]
	}
}

func parseSheetsResource(resource *ddResource, segments []string) {
	resource.id = segments[1]
}

func parseIncidents(resource *ddResource, segments []string) {
	resource.id = segments[1]
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
