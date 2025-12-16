package dd

import (
	"context"
	"net/http"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

type client interface {
	withAuth(ctx context.Context) context.Context
	getDDClient() *datadog.APIClient
	listMonitors(ctx context.Context, o ...datadogV1.ListMonitorsOptionalParameters) ([]datadogV1.Monitor, *http.Response, error)
	getMonitor(ctx context.Context, monitorId int64, o ...datadogV1.GetMonitorOptionalParameters) (datadogV1.Monitor, *http.Response, error)
	getDashboardList(ctx context.Context, listId int64) (datadogV1.DashboardList, *http.Response, error)
	getDashboard(ctx context.Context, dashboardId string) (datadogV1.Dashboard, *http.Response, error)
}

type wrapper struct {
	client *datadog.APIClient
	apiKey string
	appKey string
}

// withAuth creates a new context with DataDog API authentication from the request context
func (w wrapper) withAuth(ctx context.Context) context.Context {
	authCtx := datadog.NewDefaultContext(ctx)
	return context.WithValue(authCtx, datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {Key: w.apiKey},
		"appKeyAuth": {Key: w.appKey},
	})
}

func (w wrapper) listMonitors(ctx context.Context, o ...datadogV1.ListMonitorsOptionalParameters) ([]datadogV1.Monitor, *http.Response, error) {
	monitorsApi := datadogV1.NewMonitorsApi(w.client)
	return monitorsApi.ListMonitors(w.withAuth(ctx), o...)
}

func (w wrapper) getMonitor(ctx context.Context, monitorId int64, o ...datadogV1.GetMonitorOptionalParameters) (datadogV1.Monitor, *http.Response, error) {
	monitorsApi := datadogV1.NewMonitorsApi(w.client)
	return monitorsApi.GetMonitor(w.withAuth(ctx), monitorId, o...)
}

func (w wrapper) getDashboardList(ctx context.Context, listId int64) (datadogV1.DashboardList, *http.Response, error) {
	api := datadogV1.NewDashboardListsApi(w.client)
	return api.GetDashboardList(w.withAuth(ctx), listId)
}

func (w wrapper) getDashboard(ctx context.Context, dashboardId string) (datadogV1.Dashboard, *http.Response, error) {
	dashboardApi := datadogV1.NewDashboardsApi(w.client)
	return dashboardApi.GetDashboard(w.withAuth(ctx), dashboardId)
}

func (w wrapper) getDDClient() *datadog.APIClient {
	return w.client
}
