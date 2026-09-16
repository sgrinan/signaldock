package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type fakeExpiredSessionCleaner struct {
	deleteExpiredFunc func(context.Context) error
}

func (f *fakeExpiredSessionCleaner) DeleteExpired(ctx context.Context) error {
	if f.deleteExpiredFunc != nil {
		return f.deleteExpiredFunc(ctx)
	}

	return nil
}

func TestDeleteExpiredSessions(t *testing.T) {
	called := false
	cleaner := &fakeExpiredSessionCleaner{
		deleteExpiredFunc: func(context.Context) error {
			called = true
			return nil
		},
	}

	deleteExpiredSessions(context.Background(), cleaner, slog.Default())

	if !called {
		t.Error("deleteExpiredSessions() did not call DeleteExpired")
	}
}

func TestDeleteExpiredSessionsLogsError(t *testing.T) {
	wantErr := errors.New("database error")
	cleaner := &fakeExpiredSessionCleaner{
		deleteExpiredFunc: func(context.Context) error {
			return wantErr
		},
	}

	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))

	deleteExpiredSessions(context.Background(), cleaner, logger)

	if got := output.String(); !strings.Contains(got, wantErr.Error()) {
		t.Errorf("deleteExpiredSessions() log = %q, want error %q", got, wantErr)
	}
}
