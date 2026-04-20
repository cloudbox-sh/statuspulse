package cmd

import (
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/mcp"
)

// `statuspulse mcp serve` boots the local MCP server on stdio so AI agents
// (Claude Desktop, Claude Code, Cursor, Copilot) can drive monitors,
// incidents, status pages, and maintenance windows through typed tool calls.
// The actual implementation lives in internal/mcp; this command just plumbs
// stdio in.
//
// The command intentionally accepts no flags — the agent runtime owns the
// process lifecycle, and credentials are resolved from the same env vars +
// config file the rest of the CLI uses (run `statuspulse login` once).

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol server for AI agents",
	Long: "Run the StatusPulse MCP server so AI agents (Claude Code, Cursor, " +
		"Copilot) can manage monitors, incidents, and status pages without a browser.",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server on stdio",
	Long: "Start the MCP server on stdio. Wire this into your agent's MCP " +
		"config — see internal/mcp/README.md for copy-paste examples.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return mcp.Serve(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
}
