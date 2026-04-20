# StatusPulse

**Hosted status pages, uptime monitoring, and incident management — driven from your terminal.**

```console
$ statuspulse monitors create --name api --url https://api.example.com --interval 60
✓ monitor "api" created  (mon_2a7f1e)

$ statuspulse status
● api                 up      178ms  https://api.example.com
● www                 up       92ms  https://www.example.com
● worker-queue        degraded       tcp://queue.internal:5672

$ statuspulse incidents create --title "Elevated error rates on api" --impact minor
✓ incident inc_9b4c opened on status page "Acme"
```

Anything the dashboard at [statuspulse.cloudbox.sh](https://statuspulse.cloudbox.sh) can do, this CLI can do too — and vice versa. The same binary also runs as an [MCP server](#mcp-server), so Claude Code, Cursor, and other AI agents can manage your pages the same way you would from a shell.

> A product by **[Cloudbox](https://cloudbox.sh)** — a small NL-based studio shipping sharp developer tools. Each Cloudbox product does exactly one thing; StatusPulse's one thing is public status pages you don't have to host.

---

## Install

```sh
# npm / npx  (primary — no install step)
npx @cloudboxsh/statuspulse --help
npm i -g @cloudboxsh/statuspulse

# Homebrew  (macOS, Linux)
brew install cloudbox-sh/tap/statuspulse

# Scoop  (Windows)
scoop bucket add cloudbox https://github.com/cloudbox-sh/scoop-bucket
scoop install statuspulse

# curl | sh  (Linux, macOS)
curl -sSL https://get.cloudbox.sh/statuspulse | sh

# PowerShell  (Windows)
irm https://get.cloudbox.sh/statuspulse.ps1 | iex

# go install
go install github.com/cloudbox-sh/statuspulse@latest
```

Verify: `statuspulse --version`.

## Quickstart

```sh
statuspulse login                                                    # authenticate (browser prompt)
statuspulse pages create --name "Acme" --slug acme                   # create a public status page
statuspulse monitors create --name api \
  --url https://api.example.com --interval 60                        # add a check
statuspulse monitors attach mon_2a7f1e --page acme                   # show it on the page
statuspulse status                                                   # see current state
```

Your public page is now live at `https://statuspulse.cloudbox.sh/s/<your-org>/acme`.

---

## What StatusPulse does

- **Uptime monitoring** — HTTP / TCP / ping checks with per-monitor interval, timeout, custom headers, flexible status-code rules, and ordered body assertions (contains / regex).
- **Public status pages** — rendered at `/s/<org>/<slug>`, with six theme presets or a fully custom palette, editable logo, description, banner, and support link. Pro and Business tiers can mount pages on additional URL namespaces ("brands").
- **Incidents** — manual create / update / resolve with a full timeline. Auto-incidents open when a monitor flips to `down` and post a recovery update when it comes back up (you still write the post-mortem and resolve).
- **Scheduled maintenance windows** per page, with worker-level suppression so maintenance doesn't page you.
- **Notification channels** — email (HTML + plaintext), generic webhook, and Slack Incoming Webhook.
- **Append-only audit log** — every write captured with before/after state, exportable as CSV or JSON. Friendly toward SOC 2 CC6/CC7 and GDPR Article 30.

Pricing, free tier, and signup live at [statuspulse.cloudbox.sh](https://statuspulse.cloudbox.sh).

---

## CLI reference

All commands accept `--json` (machine-parsable output, suppresses interactive prompts) and `--api-url` (override the API endpoint — useful for self-hosted setups). Run `statuspulse <command> --help` for full flags and examples.

### Authentication

```sh
statuspulse login        # browser device-code or API-token flow
statuspulse logout       # remove local credentials
statuspulse whoami       # show the authenticated user + org
```

### Monitors

```sh
statuspulse monitors list
statuspulse monitors get <id>
statuspulse monitors create --name <name> --url <url> --interval 60 [--type http|tcp|ping]
statuspulse monitors update <id> [--name ...] [--interval ...] [--headers '{"X-Foo":"bar"}']
statuspulse monitors delete <id>
statuspulse monitors checks  <id>           # recent check results
statuspulse monitors uptime  <id> --range 30d
statuspulse monitors attach  <id> --page <page-slug>
statuspulse monitors detach  <id> --page <page-slug>
```

### Incidents

```sh
statuspulse incidents list [--page <slug>] [--status open|resolved]
statuspulse incidents get     <id>
statuspulse incidents create  --title <...> --page <slug> [--impact minor|major|critical]
statuspulse incidents update  <id> [--status investigating|identified|monitoring|resolved] [--impact ...]
statuspulse incidents post    <id> --body "we've identified the cause, rolling a fix"
statuspulse incidents resolve <id> [--body "fix deployed; apologies for the disruption"]
```

### Status pages

```sh
statuspulse pages list
statuspulse pages get    <id|slug>
statuspulse pages create --name <...> --slug <...> [--theme midnight|parchment|mono|forest|ocean|rose|custom]
statuspulse pages update <id|slug> [--name ...] [--banner "..."] [--description ...]
statuspulse pages delete <id|slug>
statuspulse pages attach <id|slug> --monitor <monitor-id>
statuspulse pages detach <id|slug> --monitor <monitor-id>
```

### Notification channels

```sh
statuspulse notifications list
statuspulse notifications create --type email|webhook|slack --target <addr|url>
statuspulse notifications update <id> [--enabled=false]
statuspulse notifications delete <id>
```

### Audit log

```sh
statuspulse audit list  [--entity monitor|page|incident|...] [--action create|update|delete]
statuspulse audit export [--format csv|json] [--since 2026-04-01] > audit.csv
```

### Public read-only (no auth required)

```sh
statuspulse public status <org>/<page-slug>   # query the public status API
```

---

## JSON output & scripting

Every command accepts `--json`. Human output is suppressed, all prompts are disabled (you must pass required values as flags), and the exit code still reflects success or failure — scripts and AI agents get stable, parseable output.

```sh
statuspulse monitors list --json | jq -r '.[] | select(.state == "down") | .name'
```

Create-style commands return an envelope:

```json
{ "ok": true, "id": "mon_2a7f1e", "monitor": { "id": "mon_2a7f1e", "name": "api", "url": "https://api.example.com", "interval": 60, "state": "up" } }
```

Error responses use a stable `code` field (`plan_interval_too_low`, `not_found`, `session_expired`, …) so callers can branch on failure type.

---

## MCP server

The same binary runs as an MCP (Model Context Protocol) server on stdio:

```sh
statuspulse mcp serve
```

Register it with your AI agent. For Claude Code:

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

18 typed tools across `monitors` / `incidents` / `pages` / `maintenance` / `ops`. Destructive operations (`delete`, `resolve`, ...) require `confirm: true` to guard against agent misfires. The MCP layer reuses the exact same REST client and authentication as the CLI — anything you can do in one, you can do in the other.

---

## Building from source

Requires Go 1.23+.

```sh
git clone https://github.com/cloudbox-sh/statuspulse
cd statuspulse
go build ./...
./statuspulse --version
```

Releases are cut via GitHub Actions on `v*` tags — see [`.goreleaser.yaml`](.goreleaser.yaml) and [`.github/workflows/release.yml`](.github/workflows/release.yml).

---

## Links

- Product: <https://statuspulse.cloudbox.sh>
- Docs: <https://statuspulse.cloudbox.sh/docs>
- `llms.txt`: <https://statuspulse.cloudbox.sh/llms.txt>
- Changelog: <https://github.com/cloudbox-sh/statuspulse/releases>
- Issues: <https://github.com/cloudbox-sh/statuspulse/issues>

## License

MIT — see [LICENSE](LICENSE).

Works with [Claude Code](https://claude.com/claude-code) · [Cursor](https://cursor.com) · [Copilot](https://github.com/features/copilot).
