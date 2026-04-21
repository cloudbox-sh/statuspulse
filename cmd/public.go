package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/config"
	"github.com/cloudbox-sh/statuspulse/internal/styles"
)

var publicCmd = &cobra.Command{
	Use:   "public",
	Short: "Read-only view of public status pages (no auth required)",
	Long: "Anonymous, unauthenticated view of any public StatusPulse page.\n\n" +
		"Distinct from `statuspulse status`, which shows YOUR org's dashboard\n" +
		"summary and requires a login. `public` works against any tenant and\n" +
		"returns the same data a visitor sees in the browser — useful from\n" +
		"scripts, cron jobs, and AI agents that need to read a page without\n" +
		"being authorised on it.\n\n" +
		"Examples:\n" +
		"  statuspulse public status cloudboxsh/cloudbox              # fetch a page\n" +
		"  statuspulse public status acme/api --json | jq .page       # pipe into jq\n" +
		"  statuspulse public status acme/api -v                      # include full details",
}

var publicStatusCmd = &cobra.Command{
	Use:   "status <id | org/slug>",
	Short: "Fetch what a public status page is advertising right now",
	Long: "Fetch the live state of a public StatusPulse page. Same data a\n" +
		"browser sees at https://statuspulse.cloudbox.sh/s/<org>/<slug>.\n\n" +
		"Accepts either:\n" +
		"  • a status-page UUID (as shown in `statuspulse pages list`) —\n" +
		"    requires `statuspulse login` so we can resolve the slug pair\n" +
		"  • an <org-slug>/<page-slug> address — no auth required, mirrors\n" +
		"    the public URL; copy-paste from a browser tab\n\n" +
		"Examples:\n" +
		"  statuspulse public status 0c765f2c-572a-45b9-b959-ef41bed4d609\n" +
		"  statuspulse public status acme/api\n" +
		"  statuspulse public status acme/api --json | jq .page.headline_status\n" +
		"  statuspulse public status othercorp/prod -v",
	Args: cobra.ExactArgs(1),
	RunE: runPublicStatus,
}

func init() {
	rootCmd.AddCommand(publicCmd)
	publicCmd.AddCommand(publicStatusCmd)
}

func runPublicStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := signalCtx()
	defer cancel()

	orgSlug, pageSlug, err := resolvePublicAddress(ctx, args[0])
	if err != nil {
		return err
	}

	// Anonymous client for the actual /api/public/* call — re-uses
	// base URL resolution so `--api-url` + env overrides still apply.
	c, _, err := newAnonClient()
	if err != nil {
		return err
	}

	st, err := c.PublicStatus(ctx, orgSlug, pageSlug)
	if err != nil {
		return handleAPIError(err, "status page", args[0])
	}
	if jsonOutput {
		return emit(st, nil)
	}

	fmt.Println(styles.Highlight.Render(st.Page.Name))
	if st.Page.Description != "" {
		fmt.Println(styles.Dim.Render(st.Page.Description))
	}
	if st.Page.HeaderText != "" {
		fmt.Println(styles.Warning.Render("▶ " + st.Page.HeaderText))
	}
	fmt.Println()

	if len(st.Monitors) == 0 {
		fmt.Println(styles.Dim.Render("no monitors on this page"))
	} else {
		for _, m := range st.Monitors {
			fmt.Printf("  %s %-32s  %s\n",
				styles.StatusGlyph(m.Status),
				truncate(m.Name, 32),
				colorUptimePct(m.Uptime90d))
		}
	}

	if len(st.Incidents) > 0 {
		fmt.Println()
		fmt.Println(styles.Accent.Render("recent incidents"))
		for _, i := range st.Incidents {
			fmt.Printf("  %s %-50s %s  %s\n",
				styles.StatusGlyph(i.Status),
				truncate(i.Title, 50),
				colorImpact(i.Impact),
				relTime(i.CreatedAt))
		}
	}
	return nil
}

// resolvePublicAddress turns the CLI arg into an (org-slug, page-slug) pair.
// If arg is a UUID we look the page up via the authenticated API (requires
// `statuspulse login`). If it's already in <org>/<slug> shape we return it
// as-is, allowing fully anonymous use.
func resolvePublicAddress(ctx context.Context, arg string) (string, string, error) {
	if uuidRe.MatchString(arg) {
		c, _, err := newClient()
		if err != nil {
			if errors.Is(err, config.ErrNotAuthenticated) {
				return "", "", fmt.Errorf("resolving a status-page UUID requires `statuspulse login`; " +
					"alternatively pass the <org>/<slug> address (as shown in the browser URL)")
			}
			return "", "", err
		}
		page, err := c.GetStatusPage(ctx, arg, 0)
		if err != nil {
			return "", "", handleAPIError(err, "status page", arg)
		}
		if page.Page.OrgSlug == "" || page.Page.Slug == "" {
			return "", "", fmt.Errorf("page %s has no public slug yet", arg)
		}
		return page.Page.OrgSlug, page.Page.Slug, nil
	}

	parts := strings.SplitN(arg, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("page address must be a UUID or <org>/<slug>, e.g. acme/prod")
	}
	return parts[0], parts[1], nil
}
