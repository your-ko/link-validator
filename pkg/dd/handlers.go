package dd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

type ddHandler func(
	ctx context.Context,
	resource ddResource,
) error

type handlerEntry struct {
	name string
	fn   ddHandler
}

func (proc *LinkProcessor) handleConnection(ctx context.Context, resource ddResource) error {
	authApi := datadogV1.NewAuthenticationApi(proc.client)
	authCtx := proc.withAuth(ctx)
	validation, _, err := authApi.Validate(authCtx)
	if err != nil {
		return fmt.Errorf("DataDog API connection failed: %w", err)
	}

	if !validation.GetValid() {
		return fmt.Errorf("invalid DataDog credentials")
	}

	return nil
}

func (proc *LinkProcessor) handleMonitors(ctx context.Context, resource ddResource) error {
	monitorsApi := datadogV1.NewMonitorsApi(proc.client)
	authCtx := proc.withAuth(ctx)

	if resource.ID == "" {
		_, _, err := monitorsApi.ListMonitors(authCtx)
		return err
	}
	monitorId, err := strconv.ParseInt(resource.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid monitor id: '%s'", resource.ID)
	}

	_, _, err = monitorsApi.GetMonitor(authCtx, monitorId)
	if err != nil {
		return err
	}
	return nil
}

func (proc *LinkProcessor) handleDashboards(ctx context.Context, resource ddResource) error {
	dashboardApi := datadogV1.NewDashboardsApi(proc.client)
	authCtx := proc.withAuth(ctx)

	if resource.ID == "" {
		_, _, err := dashboardApi.ListDashboards(authCtx)
		return err
	}
	_, _, err := dashboardApi.GetDashboard(authCtx, resource.ID)
	if err != nil {
		return err
	}
	return nil
}

//func handleLogs(ctx context.Context, id string, query string) error {
//	logsApi := datadogV1.NewLogsApi(proc.client)
//	logsApi.
//}
