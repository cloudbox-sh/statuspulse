# StatusPulse CLI

Command-line client and MCP server for [StatusPulse](https://statuspulse.cloudbox.sh) — hosted status pages from the terminal.

Anything the dashboard can do, this CLI can do too — and vice versa. Works with Claude Code, Cursor, and any MCP-aware agent via `statuspulse mcp serve`.

## Install

```sh
# npm / npx (primary — no install step)
npx @cloudbox/statuspulse --help
npm i -g @cloudbox/statuspulse

# Homebrew (macOS, Linux)
brew install cloudbox-sh/tap/statuspulse

# Scoop (Windows)
scoop bucket add cloudbox https://github.com/cloudbox-sh/scoop-bucket
scoop install statuspulse

# curl | sh (Linux, macOS)
curl -sSL https://get.cloudbox.sh/statuspulse | sh

# PowerShell (Windows)
irm https://get.cloudbox.sh/statuspulse.ps1 | iex

# go install
go install github.com/cloudbox-sh/statuspulse@latest
```

## Quickstart

```sh
statuspulse login
statuspulse pages create --name "My service"
statuspulse monitors create --name api --url https://api.example.com --interval 60
statuspulse status
```

Every command supports `--json` for scripting and agent use.

## MCP server

```sh
statuspulse mcp serve
```

Paste into your Claude Code or Cursor MCP config:

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

18 typed tools across monitors, incidents, status pages, maintenance, and ops. Destructive operations require `confirm: true`.

## Links

- Product: <https://statuspulse.cloudbox.sh>
- Docs: <https://statuspulse.cloudbox.sh/docs>
- `llms.txt`: <https://statuspulse.cloudbox.sh/llms.txt>
- Issues: <https://github.com/cloudbox-sh/statuspulse/issues>

## License

MIT — see [LICENSE](LICENSE).

A product by [Cloudbox](https://cloudbox.sh).
