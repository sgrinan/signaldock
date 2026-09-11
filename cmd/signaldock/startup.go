package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

func refreshEndpointsOnStartup(ctx context.Context, service *endpoint.Service, logger *slog.Logger) error {
	endpoints, err := service.List(ctx)
	if err != nil {
		return fmt.Errorf("load endpoints: %w", err)
	}

	for _, ep := range endpoints {
		if _, err := service.Refresh(ctx, ep.ID); err != nil {
			logger.Warn("failed to refresh endpoint on startup", "endpoint_id", ep.ID, "url", ep.URL, "error", err)
		}
	}

	return nil
}
