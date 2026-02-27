package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jakenesler/navigatorr/arrservice"
	"github.com/jakenesler/navigatorr/config"
	"github.com/jakenesler/navigatorr/internal"
	"github.com/jakenesler/navigatorr/openapi"
	"github.com/jakenesler/navigatorr/qbit"
	"github.com/jakenesler/navigatorr/tools"
	"github.com/jakenesler/navigatorr/transmission"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	configPath := flag.String("config", "", "path to config.yaml (default: ~/.config/navigatorr/config.yaml)")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	internal.Logf("loaded config with %d services", len(cfg.Services))

	// Build service registry
	registry := arrservice.NewRegistry(cfg)

	// Build OpenAPI spec store and load specs
	specStore := openapi.NewStore(cfg)
	ctx := context.Background()
	specStore.LoadAll(ctx)

	// Build Transmission client if configured
	var txClient *transmission.Client
	if cfg.Transmission.URL != "" {
		txClient = transmission.NewClient(
			cfg.Transmission.URL,
			cfg.Transmission.Username,
			cfg.Transmission.Password,
		)
		internal.Logf("transmission client configured: %s", cfg.Transmission.URL)
	}

	// Build qBittorrent client if configured
	var qbClient *qbit.Client
	if cfg.QBittorrent.URL != "" {
		qbClient = qbit.NewClient(
			cfg.QBittorrent.URL,
			cfg.QBittorrent.Username,
			cfg.QBittorrent.Password,
		)
		internal.Logf("qbittorrent client configured: %s", cfg.QBittorrent.URL)
	}

	// Create MCP server
	s := server.NewMCPServer(
		"navigatorr",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithInstructions("Navigatorrr provides tools to browse *arr service API documentation, make authenticated API calls to Sonarr/Radarr/Lidarr/Jellyseerr/etc., manage Transmission torrents, and manage qBittorrent torrents. Use list_services to see available services, search_api to find endpoints, and call_api to make requests."),
	)

	// Register all tools
	tools.RegisterAll(s, cfg, registry, specStore, txClient, qbClient)

	internal.Logf("starting navigatorr MCP server (stdio)")

	// Serve over stdio
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
