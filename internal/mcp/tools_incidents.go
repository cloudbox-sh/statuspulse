package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/wgillmer/statuspulse-cli/internal/client"
)

// registerIncidentTools wires the five incident tools.
//
// list_incidents / get_incident / create_incident / update_incident /
// post_incident_update.
//
// The "open an incident from chat" workflow is the headline MCP use-case for
// StatusPulse — keep the create_incident schema friendly (title is required;
// status / impact have sensible defaults).
func registerIncidentTools(srv *mcpserver.MCPServer, d *deps) {
	// ── list_incidents ─────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("list_incidents",
			mcp.WithDescription(
				"List incidents in the current organization. Optionally filter by status. "+
					"REST: GET /api/incidents",
			),
			mcp.WithString("status",
				mcp.Description("Filter by status: investigating | identified | monitoring | resolved. Omit to return all."),
				mcp.Enum("investigating", "identified", "monitoring", "resolved"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			incidents, err := d.api.ListIncidents(ctx, req.GetString("status", ""))
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(incidents, dashLink(d.baseURL, "/app/incidents")), nil
		},
	)

	// ── get_incident ───────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("get_incident",
			mcp.WithDescription(
				"Fetch one incident with its full timeline (updates) and the monitors it "+
					"affects. REST: GET /api/incidents/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Incident UUID.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			inc, err := d.api.GetIncident(ctx, id)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(inc, dashLink(d.baseURL, "/app/incidents/"+id)), nil
		},
	)

	// ── create_incident ────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("create_incident",
			mcp.WithDescription(
				"Open a new incident. Title is required; everything else has sensible defaults "+
					"(status=investigating, impact=minor). Pass monitor_ids to attach the "+
					"affected monitors so the dashboard timeline + auto-recovery messaging "+
					"work end-to-end. message becomes the first timeline entry. "+
					"REST: POST /api/incidents",
			),
			mcp.WithString("title", mcp.Required(), mcp.Description("Short headline shown on the public page. Plain text.")),
			mcp.WithString("status",
				mcp.Description("Initial status. Default: investigating."),
				mcp.Enum("investigating", "identified", "monitoring", "resolved"),
			),
			mcp.WithString("impact",
				mcp.Description("Impact label. Default: minor."),
				mcp.Enum("none", "minor", "major", "critical"),
			),
			mcp.WithString("message",
				mcp.Description("Optional first timeline entry shown to subscribers. Markdown-ish; renders as plain text."),
			),
			mcp.WithArray("monitor_ids",
				mcp.Description("Optional list of monitor UUIDs this incident affects."),
				mcp.WithStringItems(),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			title, err := req.RequireString("title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := client.CreateIncidentRequest{
				Title:      title,
				Status:     req.GetString("status", ""),
				Impact:     req.GetString("impact", ""),
				Message:    req.GetString("message", ""),
				MonitorIDs: req.GetStringSlice("monitor_ids", nil),
			}
			inc, err := d.api.CreateIncident(ctx, body)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(inc, dashLink(d.baseURL, "/app/incidents/"+inc.ID)), nil
		},
	)

	// ── update_incident ────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("update_incident",
			mcp.WithDescription(
				"Update an incident's top-level status or impact. Setting status=resolved "+
					"automatically stamps resolved_at server-side. To append a timeline entry, "+
					"use post_incident_update instead — that's the verb subscribers see. "+
					"REST: PUT /api/incidents/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Incident UUID.")),
			mcp.WithString("status",
				mcp.Description("New status."),
				mcp.Enum("investigating", "identified", "monitoring", "resolved"),
			),
			mcp.WithString("impact",
				mcp.Description("New impact label."),
				mcp.Enum("none", "minor", "major", "critical"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := client.UpdateIncidentRequest{}
			if v := req.GetString("status", ""); v != "" {
				body.Status = &v
			}
			if v := req.GetString("impact", ""); v != "" {
				body.Impact = &v
			}
			inc, err := d.api.UpdateIncident(ctx, id, body)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(inc, dashLink(d.baseURL, "/app/incidents/"+id)), nil
		},
	)

	// ── post_incident_update ───────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("post_incident_update",
			mcp.WithDescription(
				"Append a new entry to an incident's timeline. Subscribers receive this. "+
					"Setting status=resolved here also marks the parent incident resolved. "+
					"This is the workflow you want for ongoing comms during an incident. "+
					"REST: POST /api/incidents/{id}/updates",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Incident UUID.")),
			mcp.WithString("status",
				mcp.Required(),
				mcp.Description("Status this update represents."),
				mcp.Enum("investigating", "identified", "monitoring", "resolved"),
			),
			mcp.WithString("message",
				mcp.Required(),
				mcp.Description("Update body shown to subscribers. Be specific — what happened, what you're doing about it, when the next update is."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			status, err := req.RequireString("status")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			message, err := req.RequireString("message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			upd, err := d.api.AddIncidentUpdate(ctx, id, client.AddIncidentUpdateRequest{
				Status: status, Message: message,
			})
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(upd, dashLink(d.baseURL, "/app/incidents/"+id)), nil
		},
	)
}
