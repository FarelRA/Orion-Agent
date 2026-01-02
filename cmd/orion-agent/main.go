package main

import (
	"flag"
	"fmt"
	"os"

	"orion-agent/internal/app"
	"orion-agent/internal/infra/config"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg := config.Load(*configPath)

	// Create and run app
	application, err := app.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Run the application
	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
