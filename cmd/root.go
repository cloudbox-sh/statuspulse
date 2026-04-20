// Package cmd wires every StatusPulse CLI subcommand into a single Cobra tree.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/client"
	"github.com/cloudbox-sh/statuspulse/internal/config"
	"github.com/cloudbox-sh/statuspulse/internal/styles"
)

// Version is set via -ldflags at release time. Dev builds show "dev".
var Version = "dev"

// apiURLFlag is bound to the root --api-url persistent flag and read by
// newClient() when a subcommand builds its client.
var apiURLFlag string

// jsonOutput is bound to --json. When true, every command writes a
// machine-parsable JSON document to stdout (and suppresses the coloured
// human output + any interactive prompts).
var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:   "statuspulse",
	Short: "StatusPulse — hosted status pages from the terminal",
	Long: styles.Accent.Render("StatusPulse") + " — hosted status pages from the terminal.\n\n" +
		styles.Dim.Render("Anything the dashboard can do, this CLI can do too — and vice versa.\n"+
			"Agent-friendly: your AI assistant can drive it directly today; a native\n"+
			"MCP server is coming (`statuspulse mcp serve`).\n\n"+
			"Docs: https://statuspulse.cloudbox.sh/docs"),
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the single entry point called from main.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURLFlag, "api-url", "",
		"StatusPulse API base URL (overrides config + STATUSPULSE_API_URL)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false,
		"Emit machine-parsable JSON instead of styled tables (scripts + AI agents)")
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(styles.Accent.Render("statuspulse") + " " + Version + "\n")
}

// emit is the output helper used by every subcommand. In --json mode it
// writes a single JSON document to stdout; otherwise it runs the passed
// human-readable renderer (table, styled key/value, etc.). A nil `human`
// means the command will handle non-JSON output itself after this returns.
func emit(data any, human func()) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
	if human != nil {
		human()
	}
	return nil
}

// emitOK is shorthand for mutation responses — create/update/delete/etc.
// In --json mode it produces `{"ok":true, "id":"...", "<entity>": {...}}`;
// in human mode it prints the standard success line.
func emitOK(entity, id string, payload any, humanMsg string) error {
	if jsonOutput {
		out := map[string]any{"ok": true}
		if id != "" {
			out["id"] = id
		}
		if entity != "" && payload != nil {
			out[entity] = payload
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Println(humanMsg)
	return nil
}

// requireFlagsForJSON is called before any interactive form runs. In --json
// mode we can't show a TUI, so we tell the caller which flags to pass.
func requireFlagsForJSON(required string) error {
	if !jsonOutput {
		return nil
	}
	return fmt.Errorf("--json mode requires flags (%s) — interactive forms are disabled", required)
}

// listCmdForKind maps an entity kind (singular, as used in user-facing
// error text) to its list subcommand. Most kinds follow the plural form,
// but 'channel' is irregular (it lives under `notifications`).
func listCmdForKind(kind string) string {
	switch kind {
	case "channel":
		return "notifications list"
	case "status page":
		return "pages list"
	}
	return kind + "s list"
}

// signalCtx returns a context cancelled on Ctrl-C so long-running commands
// can shut down cleanly.
func signalCtx() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// newClient builds an authenticated API client from the resolved config.
// It returns config.ErrNotAuthenticated if no token is available — callers
// that accept unauthenticated calls (like login) should use newAnonClient.
func newClient() (*client.Client, *config.Resolved, error) {
	r, err := config.Resolve(apiURLFlag)
	if err != nil {
		return nil, nil, err
	}
	if r.Token == "" {
		return nil, r, config.ErrNotAuthenticated
	}
	return client.New(r.APIURL, r.Token), r, nil
}

// newAnonClient returns a client without requiring a token. Used by `login`.
func newAnonClient() (*client.Client, *config.Resolved, error) {
	r, err := config.Resolve(apiURLFlag)
	if err != nil {
		return nil, nil, err
	}
	return client.New(r.APIURL, r.Token), r, nil
}

// handleAPIError turns a raw API error into a friendly, context-aware one.
//
// Callers can optionally pass the entity kind + the user-supplied reference
// (id/slug) so 404s can say which thing wasn't found and how to look it up.
// Plan-limit 403s are surfaced with an upgrade hint. Session-expiry 401s
// print a dedicated stderr line because the returned error message alone
// doesn't make it obvious the token needs refreshing.
//
// Shape: `handleAPIError(err)` for generic calls, or
// `handleAPIError(err, "monitor", args[0])` to add the "monitor <id> not
// found" hint. Any error that isn't an *APIError is returned unchanged
// (network timeouts etc. already carry their own helpful message).
func handleAPIError(err error, ctx ...string) error {
	if err == nil {
		return nil
	}

	kind, ref := "", ""
	if len(ctx) >= 1 {
		kind = ctx[0]
	}
	if len(ctx) >= 2 {
		ref = ctx[1]
	}

	var ae *client.APIError
	if !errors.As(err, &ae) {
		// Plain network / transport error. Wrap with entity context if we have it.
		if kind != "" && ref != "" {
			return fmt.Errorf("%s %q: %w", kind, ref, err)
		}
		return err
	}

	switch ae.Status {
	case http.StatusUnauthorized:
		fmt.Fprintln(os.Stderr, styles.Error.Render("✗ session expired")+
			" — run "+styles.Code.Render("statuspulse login")+" to reauthenticate.")
		return errors.New("not authenticated")

	case http.StatusForbidden:
		// Plan-limit errors carry a stable `code` so we can show a targeted
		// upgrade link instead of the raw server message.
		if strings.HasPrefix(ae.Code, "plan_") {
			msg := ae.Message
			if ae.Plan != "" {
				msg += fmt.Sprintf(" (current plan: %s)", ae.Plan)
			}
			return fmt.Errorf("%s — upgrade at https://statuspulse.cloudbox.sh/pricing", msg)
		}
		return errors.New(ae.Message)

	case http.StatusNotFound:
		if kind != "" && ref != "" {
			listCmd := listCmdForKind(kind)
			return fmt.Errorf("%s %q not found — run %s to see available IDs",
				kind, ref, styles.Code.Render("statuspulse "+listCmd))
		}
		if ae.Message != "" {
			return errors.New(ae.Message)
		}
		return errors.New("not found")

	case http.StatusConflict:
		return errors.New(ae.Message)

	default:
		if ae.Status >= 500 {
			return fmt.Errorf("server error (HTTP %d) — try again in a moment", ae.Status)
		}
		if ae.Message != "" {
			return errors.New(ae.Message)
		}
		return fmt.Errorf("request failed (HTTP %d)", ae.Status)
	}
}
