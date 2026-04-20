package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/wgillmer/statuspulse-cli/internal/client"
)

// registerStatusPageTools wires the four status-page tools.
//
// list_status_pages / get_status_page / attach_monitor_to_page / detach_monitor_from_page.
//
// Create + update aren't exposed in v1 — they involve theme + branding +
// namespace decisions that benefit from human eyes. Agents can manage which
// monitors live on a page, which is the day-to-day operation.
func registerStatusPageTools(srv *mcpserver.MCPServer, d *deps) {
	// ── list_status_pages ──────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("list_status_pages",
			mcp.WithDescription(
				"List every status page in the current organization. Each result includes "+
					"the public URL slug, attached monitor IDs, theme, and enabled flag. "+
					"REST: GET /api/status-pages",
			),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pages, err := d.api.ListStatusPages(ctx)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(pages, dashLink(d.baseURL, "/app/status-page")), nil
		},
	)

	// ── get_status_page ────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("get_status_page",
			mcp.WithDescription(
				"Fetch one status page in detail: attached monitors with per-day uptime, "+
					"recent incidents touching them, and the configured headline thresholds. "+
					"REST: GET /api/status-pages/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Status page UUID.")),
			mcp.WithNumber("days",
				mcp.Description("History window in days. 7/30/60/90/180/365 are the dashboard's standard choices. Capped server-side by the org's plan retention."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := d.api.GetStatusPage(ctx, id, req.GetInt("days", 0))
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(page, dashLink(d.baseURL, "/app/status-page/"+id)), nil
		},
	)

	// ── attach_monitor_to_page ─────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("attach_monitor_to_page",
			mcp.WithDescription(
				"Attach a monitor to a status page so its current status + 90-day uptime "+
					"appear on the public view. Idempotent — re-attaching just updates the "+
					"display name + sort order. "+
					"REST: POST /api/status-pages/{id}/monitors",
			),
			mcp.WithString("page_id", mcp.Required(), mcp.Description("Status page UUID.")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor UUID.")),
			mcp.WithString("display_name",
				mcp.Description("Optional override label shown publicly instead of the monitor's own name."),
			),
			mcp.WithNumber("sort_order",
				mcp.Description("Optional position (lower = higher on the page). Defaults to the next free slot."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pageID, err := req.RequireString("page_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			monitorID, err := req.RequireString("monitor_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := client.AttachMonitorRequest{MonitorID: monitorID}
			if v := req.GetString("display_name", ""); v != "" {
				body.DisplayName = &v
			}
			if _, ok := req.GetArguments()["sort_order"]; ok {
				v := req.GetInt("sort_order", 0)
				body.SortOrder = &v
			}
			att, err := d.api.AttachMonitor(ctx, pageID, body)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(att, dashLink(d.baseURL, "/app/status-page/"+pageID)), nil
		},
	)

	// ── detach_monitor_from_page ───────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("detach_monitor_from_page",
			mcp.WithDescription(
				"Remove a monitor from a status page. The monitor itself isn't deleted; "+
					"it just stops being shown on the public view. "+
					"REST: DELETE /api/status-pages/{id}/monitors/{monitorId}",
			),
			mcp.WithString("page_id", mcp.Required(), mcp.Description("Status page UUID.")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor UUID.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pageID, err := req.RequireString("page_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			monitorID, err := req.RequireString("monitor_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := d.api.DetachMonitor(ctx, pageID, monitorID); err != nil {
				return apiErrorResult(err), nil
			}
			return mcp.NewToolResultText("monitor " + monitorID + " detached from page " + pageID), nil
		},
	)
}
