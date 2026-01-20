package dd

import (
	"context"
	"errors"
	"fmt"
	"link-validator/pkg/errs"
	"strconv"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

type ddHandler func(
	ctx context.Context,
	c client,
	resource ddResource,
) error

func handleConnection(ctx context.Context, c client, _ ddResource) error {
	validation, _, err := c.validate(ctx)
	if err != nil {
		return err
	}

	if !validation.GetValid() {
		return fmt.Errorf("invalid DataDog credentials")
	}

	return nil
}

func handleMonitors(ctx context.Context, c client, resource ddResource) error {
	if resource.id == "" {
		_, _, err := c.listMonitors(ctx)
		return err
	}
	monitorId, err := strconv.ParseInt(resource.id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid monitor id: '%s'", resource.id)
	}

	_, _, err = c.getMonitor(ctx, monitorId)
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
		_, _, err = c.getDashboardList(ctx, listId)
		return err
	}
	if resource.id != "" {
		// get particular dashboard
		_, _, err := c.getDashboard(ctx, resource.id)
		return err
	}
	return fmt.Errorf("unsupported Dashboard URL found. Please report a bug")
}

func handleNotebooks(ctx context.Context, c client, resource ddResource) error {
	notebookId, err := strconv.ParseInt(resource.id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid notebook id: '%s'", resource.id)
	}

	_, _, err = c.getNotebook(ctx, notebookId)
	return err
}

func handleSLO(ctx context.Context, c client, resource ddResource) error {
	_, _, err := c.getSLO(c.withAuth(ctx), resource.id)
	return err
}

func mapDDError(url string, err error) error {
	if err == nil {
		return nil
	}
	var ddErr datadog.GenericOpenAPIError
	if errors.As(err, &ddErr) && ddErr.ErrorMessage == "404 Not Found" {
		return errs.NewNotFound(url)
	}
	if errors.Is(err, errs.ErrNotFound) {
		return err
	}
	return err
}
