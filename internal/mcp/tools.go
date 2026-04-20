package mcp

import (
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/wgillmer/statuspulse-cli/internal/client"
)

// deps is the tiny struct every tool handler closes over. Bundling the
// client + dashboard base URL keeps every register* function's signature
// identical and makes it obvious where dashboard links come from.
type deps struct {
	api     *client.Client
	baseURL string // for dashboard links — same host as the API for now
}

// registerTools wires every v1 tool onto the given server. Each subset lives
// in its own register* function (registerMonitors, registerIncidents, …) so
// the tool surface is easy to read top-down.
func registerTools(srv *mcpserver.MCPServer, api *client.Client, baseURL string) {
	d := &deps{api: api, baseURL: baseURL}

	registerOpsTools(srv, d)
	registerMonitorTools(srv, d)
	registerIncidentTools(srv, d)
	registerStatusPageTools(srv, d)
	registerMaintenanceTools(srv, d)
}
