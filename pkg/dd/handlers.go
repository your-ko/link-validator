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
	if resource.id == "" {
		_, _, err := c.ListMonitors(ctx)
		return err
	}
	monitorId, err := strconv.ParseInt(resource.id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid monitor id: '%s'", resource.id)
	}

	_, _, err = c.GetMonitor(ctx, monitorId)
	if err != nil {
		return err
	}
	return nil
}

func handleDashboards(ctx context.Context, c client, resource ddResource) error {
	if resource.subType == "lists/manual" {
		// looking for dashboard with type == "manual_dashboard_list"
		listId, err := strconv.ParseInt(resource.id, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid dashboard list id: '%s'", resource.id)
		}
		_, _, err = c.GetDashboardList(ctx, listId)
		return err
	}
	if resource.id != "" {
		// get particular dashboard
		_, _, err := c.GetDashboard(ctx, resource.id)
		return err
	}
	return fmt.Errorf("unsupported Dashboard URL found. Please report a bug")
}

//func handleLogs(ctx context.Context, id string, query string) error {
//	logsApi := datadogV1.NewLogsApi(proc.client)
//	logsApi.
//}
