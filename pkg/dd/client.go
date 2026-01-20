package dd

import (
	"context"
	"net/http"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

type client interface {
	withAuth(ctx context.Context) context.Context
	validate(ctx context.Context) (datadogV1.AuthenticationValidationResponse, *http.Response, error)
	listMonitors(ctx context.Context, o ...datadogV1.ListMonitorsOptionalParameters) ([]datadogV1.Monitor, *http.Response, error)
	getMonitor(ctx context.Context, monitorId int64, o ...datadogV1.GetMonitorOptionalParameters) (datadogV1.Monitor, *http.Response, error)
	getDashboardList(ctx context.Context, listId int64) (datadogV1.DashboardList, *http.Response, error)
	getDashboard(ctx context.Context, dashboardId string) (datadogV1.Dashboard, *http.Response, error)
	listNotebooks(ctx context.Context, o ...datadogV1.ListNotebooksOptionalParameters) (datadogV1.NotebooksResponse, *http.Response, error)
	getNotebook(ctx context.Context, notebookId int64) (datadogV1.NotebookResponse, *http.Response, error)
	getSLO(ctx context.Context, sloId string, o ...datadogV1.GetSLOOptionalParameters) (datadogV1.SLOResponse, *http.Response, error)
}

type wrapper struct {
	client *datadog.APIClient
	apiKey string
	appKey string
}

func (w wrapper) validate(ctx context.Context) (datadogV1.AuthenticationValidationResponse, *http.Response, error) {
	authApi := datadogV1.NewAuthenticationApi(w.client)
	return authApi.Validate(w.withAuth(ctx))
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
	apiV1 := datadogV1.NewMonitorsApi(w.client)
	return apiV1.ListMonitors(w.withAuth(ctx), o...)
}

func (w wrapper) getMonitor(ctx context.Context, monitorId int64, o ...datadogV1.GetMonitorOptionalParameters) (datadogV1.Monitor, *http.Response, error) {
	apiV1 := datadogV1.NewMonitorsApi(w.client)
	return apiV1.GetMonitor(w.withAuth(ctx), monitorId, o...)
}

func (w wrapper) getDashboardList(ctx context.Context, listId int64) (datadogV1.DashboardList, *http.Response, error) {
	apiV1 := datadogV1.NewDashboardListsApi(w.client)
	return apiV1.GetDashboardList(w.withAuth(ctx), listId)
}

func (w wrapper) getDashboard(ctx context.Context, dashboardId string) (datadogV1.Dashboard, *http.Response, error) {
	apiV1 := datadogV1.NewDashboardsApi(w.client)
	return apiV1.GetDashboard(w.withAuth(ctx), dashboardId)
}

func (w wrapper) listNotebooks(ctx context.Context, o ...datadogV1.ListNotebooksOptionalParameters) (datadogV1.NotebooksResponse, *http.Response, error) {
	apiV1 := datadogV1.NewNotebooksApi(w.client)
	return apiV1.ListNotebooks(w.withAuth(ctx), o...)
}

func (w wrapper) getNotebook(ctx context.Context, notebookId int64) (datadogV1.NotebookResponse, *http.Response, error) {
	apiV1 := datadogV1.NewNotebooksApi(w.client)
	return apiV1.getNotebook(w.withAuth(ctx), notebookId)
}

func (w wrapper) getSLO(ctx context.Context, sloId string, o ...datadogV1.GetSLOOptionalParameters) (datadogV1.SLOResponse, *http.Response, error) {
	apiV1 := datadogV1.NewServiceLevelObjectivesApi(w.client)
	return apiV1.GetSLO(w.withAuth(ctx), sloId, o...)
}
