# StatusPulse MCP server

A [Model Context Protocol](https://modelcontextprotocol.io) server that lets
AI agents (Claude Desktop, Claude Code, Cursor, Copilot, …) drive StatusPulse
through typed tool calls instead of UI clicks or hand-written curl. Lives
inside the same Go binary as the CLI — start it with `statuspulse mcp serve`.

## Wire it into your agent

### Claude Desktop / Claude Code

Add to `~/.claude/mcp_servers.json` (Claude Code) or `~/Library/Application Support/Claude/claude_desktop_config.json` (Claude Desktop):

```json
{
  "mcpServers": {
    "statuspulse": {
      "command": "statuspulse",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor

`~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "statuspulse": {
      "command": "statuspulse",
      "args": ["mcp", "serve"]
    }
  }
}
```

After saving, restart the agent. The 18 tools below should appear in the
agent's tool picker.

## Auth

The MCP inherits the CLI's credentials. Run `statuspulse login` once and
both surfaces share `~/.config/cloudbox/statuspulse.json`.

For headless environments, set env vars in the agent's config block:

| Variable | Purpose |
|---|---|
| `STATUSPULSE_API_KEY` | Session token / API key |
| `STATUSPULSE_API_URL` | Override base URL (default `https://statuspulse.cloudbox.sh`) |
| `STATUSPULSE_BASIC_USER` | Optional. HTTP basic-auth user when the deploy is behind a `SITE_PASSWORD` gate |
| `STATUSPULSE_BASIC_PASS` | Optional. Matching password |

The token never reaches the LLM — it sits inside the MCP subprocess.

## Tools (v1: 18)

**Monitoring**

- `list_monitors` — every monitor with its current status
- `get_monitor` — one monitor + recent checks + linked incidents
- `create_monitor` — http / tcp / ping; supports `status_rule` and `body_assertions_json` for power users
- `update_monitor` — patch any field; `clear_status_rule:true` + `status_rule:""` removes a rule
- `delete_monitor` — destructive; requires `confirm:true`

**Incidents** (the headline workflow — open status updates from chat)

- `list_incidents`, `get_incident`
- `create_incident` — title required; defaults `status=investigating`, `impact=minor`; can attach `monitor_ids`
- `update_incident` — change top-level status / impact (resolve = stamp `resolved_at`)
- `post_incident_update` — append to the timeline; subscribers receive this

**Status pages**

- `list_status_pages`, `get_status_page`
- `attach_monitor_to_page` / `detach_monitor_from_page`

(create / update / delete intentionally not exposed in v1 — too many
branding / namespace decisions benefit from human eyes)

**Maintenance**

- `schedule_maintenance` — RFC 3339 `starts_at` / `ends_at`; suppresses
  auto-incidents + notifications for monitors on the page during the window
- `cancel_maintenance` — destructive; requires `confirm:true`

**Ops**

- `whoami` — current user + org + plan limits
- `get_public_status` — read a public page without auth (e.g. when asking
  about a page the user doesn't own)

## Safety

Every destructive tool (`delete_monitor`, `cancel_maintenance`) requires the
agent to set `confirm: true`. The tool description tells the model to
surface this as an explicit yes/no to the user before flipping the flag.

Every successful tool result includes a dashboard link
(`→ https://statuspulse.cloudbox.sh/app/...`) so the human reading the
agent's transcript can verify the change without leaving their chat.

## Out of scope (deferred)

- Notification channel CRUD
- Audit log queries / export
- Status page create / update / delete (theme + branding + namespace are
  better managed in the dashboard)
- MCP `resources` and `prompts`
- HTTP / SSE transport (stdio only — every current agent runtime supports it)
