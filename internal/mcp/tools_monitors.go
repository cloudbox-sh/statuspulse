package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/cloudbox-sh/statuspulse/internal/client"
)

// registerMonitorTools wires the five monitor tools.
//
// list_monitors / get_monitor / create_monitor / update_monitor / delete_monitor
//
// create + update intentionally expose a small input schema (name, type,
// target, interval, method, expected_status). Power features (status_rule,
// custom headers, body assertions) are accepted as raw JSON strings via the
// `headers_json` and `body_assertions_json` fields so an agent can still set
// them without bloating the schema for the common case.
func registerMonitorTools(srv *mcpserver.MCPServer, d *deps) {
	// ── list_monitors ──────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("list_monitors",
			mcp.WithDescription(
				"List every monitor in the current organization with its current status. "+
					"Returns id, name, type (http|tcp|ping), target, interval, current_status, last_checked_at. "+
					"REST: GET /api/monitors",
			),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ms, err := d.api.ListMonitors(ctx)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(ms, dashLink(d.baseURL, "/app/monitors")), nil
		},
	)

	// ── get_monitor ────────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("get_monitor",
			mcp.WithDescription(
				"Fetch one monitor with its 20 most recent check results and any "+
					"incidents it's been linked to in the last 90 days. "+
					"REST: GET /api/monitors/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Monitor UUID.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			m, err := d.api.GetMonitor(ctx, id)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(m, dashLink(d.baseURL, "/app/monitors/"+id)), nil
		},
	)

	// ── create_monitor ─────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("create_monitor",
			mcp.WithDescription(
				"Create a new monitor. The minimum is name + type + target. Type must be "+
					"one of http|tcp|ping. For HTTP, target is a URL; for TCP, host:port; "+
					"for ping, a hostname. Plan limits apply (Hobby caps at 5 monitors, "+
					"Pro at 25, Business at 100; each plan has a minimum interval). "+
					"REST: POST /api/monitors",
			),
			mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable name. Shows up on the dashboard and status pages.")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("One of: http, tcp, ping."),
				mcp.Enum("http", "tcp", "ping"),
			),
			mcp.WithString("target", mcp.Required(),
				mcp.Description("URL for http, host:port for tcp, hostname for ping."),
			),
			mcp.WithNumber("interval_seconds",
				mcp.Description("How often to check (seconds). Default 60. Minimum is plan-dependent (Hobby 60s, Pro 30s, Business 10s)."),
			),
			mcp.WithNumber("timeout_seconds",
				mcp.Description("Per-check timeout (seconds). Default 10."),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method (GET/POST/HEAD). Defaults to GET. Ignored for tcp/ping."),
				mcp.Enum("GET", "POST", "HEAD"),
			),
			mcp.WithNumber("expected_status",
				mcp.Description("Expected HTTP status code. Defaults to 200. Ignored if status_rule is set."),
			),
			mcp.WithString("status_rule",
				mcp.Description("Optional flexible status matcher. Examples: \"2xx\", \"200-299\", \"200,201,204\", \"2xx,301\". When set, takes precedence over expected_status."),
			),
			mcp.WithString("headers_json",
				mcp.Description("Optional JSON object of HTTP request headers, e.g. {\"Authorization\":\"Bearer xyz\"}. Ignored for tcp/ping."),
			),
			mcp.WithString("body_assertions_json",
				mcp.Description("Optional JSON array of body assertions, each {\"type\":\"contains|not_contains|regex\",\"value\":\"...\"}. Checked against the first 128 KB of the response body."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			typ, err := req.RequireString("type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			target, err := req.RequireString("target")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body := client.CreateMonitorRequest{
				Name:            name,
				Type:            typ,
				Target:          target,
				IntervalSeconds: req.GetInt("interval_seconds", 0),
				TimeoutSeconds:  req.GetInt("timeout_seconds", 0),
				Method:          req.GetString("method", ""),
				ExpectedStatus:  req.GetInt("expected_status", 0),
			}
			if rule := req.GetString("status_rule", ""); rule != "" {
				body.StatusRule = &rule
			}
			if h := req.GetString("headers_json", ""); h != "" {
				if !json.Valid([]byte(h)) {
					return mcp.NewToolResultError("headers_json is not valid JSON"), nil
				}
				body.Headers = json.RawMessage(h)
			}
			if a := req.GetString("body_assertions_json", ""); a != "" {
				if !json.Valid([]byte(a)) {
					return mcp.NewToolResultError("body_assertions_json is not valid JSON"), nil
				}
				body.BodyAssertions = json.RawMessage(a)
			}

			m, err := d.api.CreateMonitor(ctx, body)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(m, dashLink(d.baseURL, "/app/monitors/"+m.ID)), nil
		},
	)

	// ── update_monitor ─────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("update_monitor",
			mcp.WithDescription(
				"Update a monitor in place. Only fields you pass will change; everything "+
					"else is left alone. Pass status_rule=\"\" with clear_status_rule=true to "+
					"unset the rule and fall back to expected_status. "+
					"REST: PUT /api/monitors/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Monitor UUID.")),
			mcp.WithString("name", mcp.Description("New name.")),
			mcp.WithString("target", mcp.Description("New target.")),
			mcp.WithString("type", mcp.Description("New type."), mcp.Enum("http", "tcp", "ping")),
			mcp.WithNumber("interval_seconds", mcp.Description("New check interval (seconds).")),
			mcp.WithNumber("timeout_seconds", mcp.Description("New timeout (seconds).")),
			mcp.WithString("method", mcp.Description("New HTTP method."), mcp.Enum("GET", "POST", "HEAD")),
			mcp.WithNumber("expected_status", mcp.Description("New expected HTTP status code.")),
			mcp.WithString("status_rule", mcp.Description("New status rule. Pass clear_status_rule:true alongside an empty string to unset.")),
			mcp.WithBoolean("clear_status_rule", mcp.Description("Set to true together with status_rule:\"\" to clear the rule.")),
			mcp.WithString("headers_json", mcp.Description("New headers JSON object.")),
			mcp.WithString("body_assertions_json", mcp.Description("New body_assertions JSON array.")),
			mcp.WithBoolean("enabled", mcp.Description("Pause/resume the monitor without deleting it.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body := client.UpdateMonitorRequest{
				ClearStatusRule: req.GetBool("clear_status_rule", false),
			}
			if v := req.GetString("name", ""); v != "" {
				body.Name = &v
			}
			if v := req.GetString("target", ""); v != "" {
				body.Target = &v
			}
			if v := req.GetString("type", ""); v != "" {
				body.Type = &v
			}
			if v := req.GetInt("interval_seconds", 0); v > 0 {
				body.IntervalSeconds = &v
			}
			if v := req.GetInt("timeout_seconds", 0); v > 0 {
				body.TimeoutSeconds = &v
			}
			if v := req.GetString("method", ""); v != "" {
				body.Method = &v
			}
			if v := req.GetInt("expected_status", 0); v > 0 {
				body.ExpectedStatus = &v
			}
			if _, ok := req.GetArguments()["status_rule"]; ok {
				v := req.GetString("status_rule", "")
				body.StatusRule = &v
			}
			if h := req.GetString("headers_json", ""); h != "" {
				if !json.Valid([]byte(h)) {
					return mcp.NewToolResultError("headers_json is not valid JSON"), nil
				}
				raw := json.RawMessage(h)
				body.Headers = &raw
			}
			if a := req.GetString("body_assertions_json", ""); a != "" {
				if !json.Valid([]byte(a)) {
					return mcp.NewToolResultError("body_assertions_json is not valid JSON"), nil
				}
				raw := json.RawMessage(a)
				body.BodyAssertions = &raw
			}
			if _, ok := req.GetArguments()["enabled"]; ok {
				v := req.GetBool("enabled", true)
				body.Enabled = &v
			}
			m, err := d.api.UpdateMonitor(ctx, id, body)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(m, dashLink(d.baseURL, "/app/monitors/"+id)), nil
		},
	)

	// ── delete_monitor ─────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("delete_monitor",
			mcp.WithDescription(
				"Permanently delete a monitor and all of its check history. This also "+
					"detaches the monitor from any status pages and unlinks it from incidents. "+
					"Requires confirm:true (see the field's description). "+
					"REST: DELETE /api/monitors/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Monitor UUID.")),
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
			if err := d.api.DeleteMonitor(ctx, id); err != nil {
				return apiErrorResult(err), nil
			}
			return mcp.NewToolResultText("monitor deleted: " + id), nil
		},
	)
}
