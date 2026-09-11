package main

import (
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)

	if err := run(logger); err != nil {
		logger.Error("SignalDock failed", "error", err)
		os.Exit(1)
	}
}
