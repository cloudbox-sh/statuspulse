package mcp

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/cloudbox-sh/statuspulse/internal/client"
)

// registerMaintenanceTools wires schedule_maintenance + cancel_maintenance.
//
// While an active window covers a status page, the worker suppresses
// auto-incidents and state-change notifications for the page's monitors —
// no false-alarm pages during planned downtime.
func registerMaintenanceTools(srv *mcpserver.MCPServer, d *deps) {
	// ── schedule_maintenance ───────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("schedule_maintenance",
			mcp.WithDescription(
				"Schedule a maintenance window on a status page. The public page renders "+
					"a warn-coloured banner from starts_at through ends_at, and the worker "+
					"suppresses auto-incidents + state-change notifications for monitors on "+
					"that page during the window. starts_at/ends_at are RFC 3339 in UTC "+
					"(e.g. 2026-04-25T02:00:00Z). "+
					"REST: POST /api/status-pages/{id}/maintenance",
			),
			mcp.WithString("page_id", mcp.Required(), mcp.Description("Status page UUID.")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Headline shown on the public banner. Plain text.")),
			mcp.WithString("description",
				mcp.Description("Optional longer description. Shown beneath the title on the banner."),
			),
			mcp.WithString("starts_at",
				mcp.Required(),
				mcp.Description("RFC 3339 UTC start timestamp (e.g. 2026-04-25T02:00:00Z)."),
			),
			mcp.WithString("ends_at",
				mcp.Required(),
				mcp.Description("RFC 3339 UTC end timestamp. Must be strictly after starts_at."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pageID, err := req.RequireString("page_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			title, err := req.RequireString("title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			startsRaw, err := req.RequireString("starts_at")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			endsRaw, err := req.RequireString("ends_at")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			starts, err := time.Parse(time.RFC3339, startsRaw)
			if err != nil {
				return mcp.NewToolResultErrorf("starts_at: %v (use RFC 3339, e.g. 2026-04-25T02:00:00Z)", err), nil
			}
			ends, err := time.Parse(time.RFC3339, endsRaw)
			if err != nil {
				return mcp.NewToolResultErrorf("ends_at: %v (use RFC 3339, e.g. 2026-04-25T03:00:00Z)", err), nil
			}
			win, err := d.api.ScheduleMaintenance(ctx, pageID, client.ScheduleMaintenanceRequest{
				Title:       title,
				Description: req.GetString("description", ""),
				StartsAt:    starts,
				EndsAt:      ends,
			})
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(win, dashLink(d.baseURL, "/app/status-page/"+pageID)), nil
		},
	)

	// ── cancel_maintenance ─────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("cancel_maintenance",
			mcp.WithDescription(
				"Cancel (delete) a scheduled maintenance window. Removes the public banner "+
					"and re-enables auto-incidents + notifications for affected monitors. "+
					"Requires confirm:true. "+
					"REST: DELETE /api/maintenance/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Maintenance window UUID.")),
			confirmField(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if r := requireConfirm(req); r != nil {
				return r, nil
			}
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := d.api.CancelMaintenance(ctx, id); err != nil {
				return apiErrorResult(err), nil
			}
			return mcp.NewToolResultText("maintenance window cancelled: " + id), nil
		},
	)
}
