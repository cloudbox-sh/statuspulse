package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// registerOpsTools wires identity + read-only public-status tools.
func registerOpsTools(srv *mcpserver.MCPServer, d *deps) {
	// ── whoami ─────────────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("whoami",
			mcp.WithDescription(
				"Return the authenticated user, the active organization, and the plan limits "+
					"(monitor cap, status-page cap, minimum interval, history retention). "+
					"Call this first when an agent boots up to see what the current credentials can do. "+
					"REST: GET /api/auth/me",
			),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			me, err := d.api.Me(ctx)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(me, dashLink(d.baseURL, "/app/settings")), nil
		},
	)

	// ── get_public_status ──────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("get_public_status",
			mcp.WithDescription(
				"Read a public status page without authentication. Useful when the user asks "+
					"about a page they don't necessarily own, or for quick health checks. "+
					"Returns the headline status (up/degraded/outage), each monitor's status, "+
					"open incidents, and any active maintenance windows. "+
					"REST: GET /api/public/status/{org}/{slug}",
			),
			mcp.WithString("org_slug",
				mcp.Required(),
				mcp.Description("Organization slug — the first segment of the public URL (/s/{org}/{slug})."),
			),
			mcp.WithString("page_slug",
				mcp.Required(),
				mcp.Description("Status page slug — the second segment of the public URL."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := req.RequireString("org_slug")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			slug, err := req.RequireString("page_slug")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			status, err := d.api.PublicStatus(ctx, org, slug)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(status, dashLink(d.baseURL, "/s/"+org+"/"+slug)), nil
		},
	)
}
