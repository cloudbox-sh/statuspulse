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
	Short: "MCP server for AI agents (you don't invoke this by hand)",
	Long: "Model Context Protocol server so AI agents (Claude Code, Cursor,\n" +
		"Copilot) can manage monitors, incidents, and status pages through\n" +
		"typed tool calls — the same surface the CLI and dashboard use.\n\n" +
		"You normally DO NOT run `statuspulse mcp serve` yourself. Instead,\n" +
		"drop this block into your agent's MCP config and the agent starts\n" +
		"the server for you on each session:\n\n" +
		"  {\n" +
		"    \"mcpServers\": {\n" +
		"      \"statuspulse\": {\n" +
		"        \"command\": \"statuspulse\",\n" +
		"        \"args\": [\"mcp\", \"serve\"]\n" +
		"      }\n" +
		"    }\n" +
		"  }\n\n" +
		"Typical paths for that config:\n" +
		"  Claude Code / Desktop  ~/.claude/mcp_servers.json\n" +
		"  Cursor                 ~/.cursor/mcp.json\n\n" +
		"Run `statuspulse login` once before the first agent session — the\n" +
		"MCP server reuses the CLI's session token.",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the MCP server on stdio (invoked by your AI agent, not by you)",
	Long: "Run the MCP server on stdio — reads JSON-RPC on stdin, writes\n" +
		"on stdout. This is what your AI agent launches in the background\n" +
		"when you configure it; running it manually in a terminal leaves\n" +
		"you staring at a blocked process waiting for JSON-RPC that will\n" +
		"never come.\n\n" +
		"See `statuspulse mcp --help` for the config block to paste into\n" +
		"your agent. Full tool reference + examples: internal/mcp/README.md\n" +
		"in github.com/cloudbox-sh/statuspulse.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return mcp.Serve(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
}
