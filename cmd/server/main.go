package main

import (
	"context"
	"fmt"
	"os"

	"git.999.haus/chris/DocuMCP-go/internal/app"
	"git.999.haus/chris/DocuMCP-go/internal/config"
)

// Set via ldflags at build time.
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Override version from build ldflags if set.
	if version != "dev" {
		cfg.DocuMCP.ServerVersion = version
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	application, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := application.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup error: %v\n", err)
		}
	}()

	application.Logger.Info("DocuMCP starting",
		"version", cfg.DocuMCP.ServerVersion,
		"build_time", buildTime,
		"env", cfg.App.Env,
	)

	return application.Start(context.Background())
}
