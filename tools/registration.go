package tools

import (
	"github.com/jakenesler/navigatorr/arrservice"
	"github.com/jakenesler/navigatorr/config"
	"github.com/jakenesler/navigatorr/openapi"
	"github.com/jakenesler/navigatorr/qbit"
	"github.com/jakenesler/navigatorr/transmission"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers all tools with the MCP server.
func RegisterAll(s *server.MCPServer, cfg *config.Config, registry *arrservice.Registry, specStore *openapi.Store, txClient *transmission.Client, qbClient *qbit.Client) {
	registerDocTools(s, registry, specStore)
	registerAPICallTool(s, registry)
	if txClient != nil {
		registerTransmissionTools(s, txClient)
	}
	if qbClient != nil {
		registerQbitTools(s, qbClient)
	}
}
