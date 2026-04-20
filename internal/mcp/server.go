// Package mcp hosts the StatusPulse Model Context Protocol server. It runs
// as a stdio subprocess of the `statuspulse` CLI binary (`statuspulse mcp
// serve`) so AI agents (Claude Desktop, Claude Code, Cursor, …) can manage
// monitors, incidents, status pages, and maintenance windows through typed
// tool calls instead of UI clicks or hand-written curl.
//
// The package owns no state of its own — it borrows the CLI's existing
// internal/client and internal/config packages so credentials, base URL, and
// API shapes stay in lockstep with the rest of the binary.
package mcp

import (
	"context"
	"fmt"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/cloudbox-sh/statuspulse/internal/client"
	"github.com/cloudbox-sh/statuspulse/internal/config"
)

// Version is the MCP server version reported on initialize. Bumped manually
// when the tool surface changes meaningfully.
const Version = "1.0.0"

// Serve loads credentials, builds an authenticated API client, registers all
// tools, and runs the MCP server on stdio until ctx is cancelled or the
// agent disconnects.
//
// Auth resolution mirrors the rest of the CLI:
//  1. STATUSPULSE_API_URL / STATUSPULSE_API_KEY env vars
//  2. ~/.config/cloudbox/statuspulse.json (written by `statuspulse login`)
//  3. Built-in defaults (prod URL, no token — fails fast if no token found)
//
// If the deploy is sitting behind the temporary SITE_PASSWORD basic-auth
// gate, two extra env vars layer on top:
//   - STATUSPULSE_BASIC_USER
//   - STATUSPULSE_BASIC_PASS
func Serve(ctx context.Context) error {
	cfg, err := config.Resolve("")
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}
	if cfg.Token == "" {
		return fmt.Errorf("%w (the MCP server inherits the CLI's credentials)", config.ErrNotAuthenticated)
	}

	api := client.New(cfg.APIURL, cfg.Token).
		WithBasicAuth(os.Getenv("STATUSPULSE_BASIC_USER"), os.Getenv("STATUSPULSE_BASIC_PASS"))

	srv := mcpserver.NewMCPServer(
		"statuspulse",
		Version,
		mcpserver.WithToolCapabilities(false), // we expose tools but don't push list-changed notifications
		mcpserver.WithRecovery(),              // a panic in one tool shouldn't kill the whole server
	)

	registerTools(srv, api, cfg.APIURL)

	// Surface a one-line greeting on stderr — visible in the agent's debug
	// log but not on stdout where it would corrupt the JSON-RPC stream.
	fmt.Fprintf(os.Stderr, "statuspulse MCP %s ready (api=%s)\n", Version, cfg.APIURL)
	return mcpserver.ServeStdio(srv)
}
