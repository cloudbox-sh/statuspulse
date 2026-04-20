package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wgillmer/statuspulse-cli/internal/styles"
)

var publicCmd = &cobra.Command{
	Use:   "public",
	Short: "Read-only view of public status pages (no auth required)",
}

var publicStatusCmd = &cobra.Command{
	Use:   "status <org>/<slug>",
	Short: "Fetch what a public status page is advertising right now",
	Args:  cobra.ExactArgs(1),
	RunE:  runPublicStatus,
}

func init() {
	rootCmd.AddCommand(publicCmd)
	publicCmd.AddCommand(publicStatusCmd)
}

func runPublicStatus(cmd *cobra.Command, args []string) error {
	// Page addresses are {org-slug}/{page-slug} (matches the /s/{org}/{slug}
	// URL you see in the browser).
	parts := strings.SplitN(args[0], "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("page address must be <org>/<slug>, e.g. acme/prod")
	}
	orgSlug, pageSlug := parts[0], parts[1]

	// No auth is needed, but we re-use the same base URL resolution so
	// `--api-url` and env overrides still apply (useful for self-hosted).
	c, _, err := newAnonClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

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
