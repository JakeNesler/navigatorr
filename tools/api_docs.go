package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jakenesler/navigatorr/arrservice"
	"github.com/jakenesler/navigatorr/openapi"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDocTools(s *server.MCPServer, registry *arrservice.Registry, store *openapi.Store) {
	// list_services
	s.AddTool(
		mcp.NewTool("list_services",
			mcp.WithDescription("List all configured *arr services with their URLs and status"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleListServices(ctx, registry, store)
		},
	)

	// list_endpoints
	s.AddTool(
		mcp.NewTool("list_endpoints",
			mcp.WithDescription("List API endpoints for a service, optionally filtered by tag or HTTP method"),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name (e.g. sonarr, radarr)")),
			mcp.WithString("tag", mcp.Description("Filter by API tag/category")),
			mcp.WithString("method", mcp.Description("Filter by HTTP method (GET, POST, PUT, DELETE)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			svcName := mcp.ParseString(req, "service", "")
			tag := mcp.ParseString(req, "tag", "")
			method := strings.ToUpper(mcp.ParseString(req, "method", ""))
			return handleListEndpoints(ctx, store, svcName, tag, method)
		},
	)

	// search_api
	s.AddTool(
		mcp.NewTool("search_api",
			mcp.WithDescription("Full-text search across all API specs. Searches endpoint paths, summaries, descriptions, and tags."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
			mcp.WithString("service", mcp.Description("Limit search to a specific service")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := mcp.ParseString(req, "query", "")
			svcName := mcp.ParseString(req, "service", "")
			return handleSearchAPI(ctx, store, query, svcName)
		},
	)

	// get_endpoint_details
	s.AddTool(
		mcp.NewTool("get_endpoint_details",
			mcp.WithDescription("Get full details for a specific API endpoint including parameters, request body schema, and response schema"),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name")),
			mcp.WithString("path", mcp.Required(), mcp.Description("Endpoint path (e.g. /series)")),
			mcp.WithString("method", mcp.Description("HTTP method (defaults to GET)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			svcName := mcp.ParseString(req, "service", "")
			path := mcp.ParseString(req, "path", "")
			method := strings.ToUpper(mcp.ParseString(req, "method", "GET"))
			return handleGetEndpointDetails(ctx, store, svcName, path, method)
		},
	)

	// refresh_api_specs
	s.AddTool(
		mcp.NewTool("refresh_api_specs",
			mcp.WithDescription("Force re-fetch and re-parse OpenAPI specs for all services or a specific service"),
			mcp.WithString("service", mcp.Description("Service name to refresh (omit for all)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			svcName := mcp.ParseString(req, "service", "")
			return handleRefreshSpecs(ctx, store, svcName)
		},
	)
}

func handleListServices(_ context.Context, registry *arrservice.Registry, store *openapi.Store) (*mcp.CallToolResult, error) {
	type svcInfo struct {
		Name       string `json:"name"`
		URL        string `json:"url"`
		AuthMethod string `json:"auth_method"`
		HasSpec    bool   `json:"has_spec"`
		Endpoints  int    `json:"endpoints,omitempty"`
	}

	var services []svcInfo
	for _, name := range registry.List() {
		svc, _ := registry.Get(name)
		info := svcInfo{
			Name:       name,
			URL:        svc.Config.URL,
			AuthMethod: svc.Config.AuthMethod,
		}
		if idx := store.GetIndex(name); idx != nil {
			info.HasSpec = true
			info.Endpoints = idx.Count()
		}
		services = append(services, info)
	}

	data, _ := json.MarshalIndent(services, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

func handleListEndpoints(_ context.Context, store *openapi.Store, svcName, tag, method string) (*mcp.CallToolResult, error) {
	if svcName == "" {
		return mcp.NewToolResultError("service is required"), nil
	}

	idx := store.GetIndex(svcName)
	if idx == nil {
		return mcp.NewToolResultError(fmt.Sprintf("no API spec loaded for %q — try refresh_api_specs first", svcName)), nil
	}

	endpoints := idx.Filter(tag, method)
	if len(endpoints) == 0 {
		return mcp.NewToolResultText("No endpoints match the given filters."), nil
	}

	// Compact listing
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s API Endpoints (%d)\n\n", svcName, len(endpoints)))

	// Group by tag
	tagMap := make(map[string][]openapi.EndpointSummary)
	for _, ep := range endpoints {
		t := ep.Tag
		if t == "" {
			t = "untagged"
		}
		tagMap[t] = append(tagMap[t], ep)
	}

	for t, eps := range tagMap {
		sb.WriteString(fmt.Sprintf("## %s\n", t))
		for _, ep := range eps {
			sb.WriteString(fmt.Sprintf("- %s %s — %s\n", ep.Method, ep.Path, ep.Summary))
		}
		sb.WriteString("\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleSearchAPI(_ context.Context, store *openapi.Store, query, svcName string) (*mcp.CallToolResult, error) {
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	results := store.Search(query, svcName)
	if len(results) == 0 {
		return mcp.NewToolResultText("No results found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Search results for %q (%d matches)\n\n", query, len(results)))
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("**[%s]** %s %s\n", r.Service, r.Method, r.Path))
		if r.Summary != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", r.Summary))
		}
		sb.WriteString("\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func handleGetEndpointDetails(_ context.Context, store *openapi.Store, svcName, path, method string) (*mcp.CallToolResult, error) {
	if svcName == "" || path == "" {
		return mcp.NewToolResultError("service and path are required"), nil
	}

	idx := store.GetIndex(svcName)
	if idx == nil {
		return mcp.NewToolResultError(fmt.Sprintf("no API spec loaded for %q", svcName)), nil
	}

	detail, err := idx.GetDetail(path, method)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, _ := json.MarshalIndent(detail, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

func handleRefreshSpecs(ctx context.Context, store *openapi.Store, svcName string) (*mcp.CallToolResult, error) {
	if svcName != "" {
		if err := store.Refresh(ctx, svcName); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to refresh %s: %v", svcName, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Refreshed spec for %s", svcName)), nil
	}

	errors := store.RefreshAll(ctx)
	if len(errors) > 0 {
		var sb strings.Builder
		sb.WriteString("Refresh completed with errors:\n")
		for svc, err := range errors {
			sb.WriteString(fmt.Sprintf("- %s: %v\n", svc, err))
		}
		return mcp.NewToolResultText(sb.String()), nil
	}

	return mcp.NewToolResultText("All specs refreshed successfully"), nil
}
