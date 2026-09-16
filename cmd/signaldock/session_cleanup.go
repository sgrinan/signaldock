package main

import (
	"context"
	"log/slog"
	"time"
)

const sessionCleanupInterval = time.Hour

type expiredSessionCleaner interface {
	DeleteExpired(context.Context) error
}

func runSessionCleanup(ctx context.Context, cleaner expiredSessionCleaner, logger *slog.Logger) {
	deleteExpiredSessions(ctx, cleaner, logger)

	ticker := time.NewTicker(sessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleteExpiredSessions(ctx, cleaner, logger)
		}
	}
}

func deleteExpiredSessions(ctx context.Context, cleaner expiredSessionCleaner, logger *slog.Logger) {
	if err := cleaner.DeleteExpired(ctx); err != nil && ctx.Err() == nil {
		logger.Error("failed to delete expired sessions", "error", err)
	}
}
