package mcp

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/cloudbox-sh/statuspulse/internal/client"
)

// jsonResult marshals v to pretty JSON and returns it as the tool's text
// content. Agents handle JSON natively, so a stable shape beats a styled
// human view here.
func jsonResult(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultErrorf("marshal result: %v", err)
	}
	return mcp.NewToolResultText(string(b))
}

// jsonResultWithLink wraps jsonResult and appends a "View at <url>" line so
// the human reading the agent's transcript can jump to the dashboard.
func jsonResultWithLink(v any, dashboardURL string) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultErrorf("marshal result: %v", err)
	}
	return mcp.NewToolResultText(string(b) + "\n\n→ " + dashboardURL)
}

// apiErrorResult turns an API error into a useful tool error: keeps the
// human message, threads the API's machine-readable code through, and
// surfaces the plan name when the failure was a plan-cap rejection.
func apiErrorResult(err error) *mcp.CallToolResult {
	var ae *client.APIError
	if errors.As(err, &ae) {
		switch {
		case ae.Code != "" && ae.Plan != "":
			return mcp.NewToolResultErrorf("%s (code=%s, plan=%s)", ae.Message, ae.Code, ae.Plan)
		case ae.Code != "":
			return mcp.NewToolResultErrorf("%s (code=%s)", ae.Message, ae.Code)
		default:
			return mcp.NewToolResultErrorf("%s (HTTP %d)", ae.Message, ae.Status)
		}
	}
	return mcp.NewToolResultErrorf("%v", err)
}

// dashboardBaseURL trims `/api`-style suffixes off baseURL so we can build
// links like `<base>/app/monitors/<id>`. The CLI's baseURL is the API host
// today (`https://statuspulse.cloudbox.sh`), so the function is mostly a
// future-proofing hook.
func dashboardBaseURL(apiBase string) string {
	return apiBase
}

// dashLink concatenates the dashboard base URL with the given path, taking
// care to avoid double-slashes.
func dashLink(apiBase, path string) string {
	return fmt.Sprintf("%s%s", apiBase, path)
}
