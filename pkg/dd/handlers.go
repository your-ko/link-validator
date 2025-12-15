package dd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

type ddHandler func(
	ctx context.Context,
	c client,
	resource ddResource,
) error

type handlerEntry struct {
	name string
	fn   ddHandler
}

func handleConnection(ctx context.Context, c client, resource ddResource) error {
	authApi := datadogV1.NewAuthenticationApi(c.getDDClient())
	validation, _, err := authApi.Validate(ctx)
	if err != nil {
		return fmt.Errorf("DataDog API connection failed: %w", err)
	}

	if !validation.GetValid() {
		return fmt.Errorf("invalid DataDog credentials")
	}

	return nil
}

func handleMonitors(ctx context.Context, c client, resource ddResource) error {
	if resource.ID == "" {
		_, _, err := c.ListMonitors(ctx)
		return err
	}
	monitorId, err := strconv.ParseInt(resource.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid monitor id: '%s'", resource.ID)
	}

	_, _, err = c.GetMonitor(ctx, monitorId)
	if err != nil {
		return err
	}
	return nil
}

func handleDashboards(ctx context.Context, c client, resource ddResource) error {
	if resource.ID == "" {
		_, _, err := c.ListDashboards(ctx)
		return err
	}
	_, _, err := c.GetDashboard(ctx, resource.ID)
	if err != nil {
		return err
	}
	return nil
}

//func handleLogs(ctx context.Context, id string, query string) error {
//	logsApi := datadogV1.NewLogsApi(proc.client)
//	logsApi.
//}
