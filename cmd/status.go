package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wgillmer/statuspulse-cli/internal/styles"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a one-screen health summary for your org",
	Long: "Prints total/up/down monitor counts and active incidents.\n" +
		"Exits non-zero when anything is degraded — safe to pipe into a shell script.",
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	stats, err := c.Dashboard(ctx)
	if err != nil {
		return handleAPIError(err)
	}

	degraded := stats.DownCount > 0 || stats.IncidentsActive > 0

	if err := emit(stats, func() {
		headline := styles.Success.Render("● all systems operational")
		if degraded {
			headline = styles.Error.Render("● service degraded")
		}
		fmt.Println(headline)
		fmt.Println()

		fmt.Printf("  %s %s monitors total\n",
			styles.StatusGlyph("unknown"), styles.Highlight.Render(fmt.Sprintf("%d", stats.MonitorsCount)))
		fmt.Printf("  %s %s up\n",
			styles.StatusGlyph("up"), styles.Success.Render(fmt.Sprintf("%d", stats.UpCount)))
		fmt.Printf("  %s %s down\n",
			styles.StatusGlyph("down"), styles.Error.Render(fmt.Sprintf("%d", stats.DownCount)))
		fmt.Printf("  %s %s active incidents\n",
			styles.StatusGlyph("investigating"), styles.Warning.Render(fmt.Sprintf("%d", stats.IncidentsActive)))
	}); err != nil {
		return err
	}

	// Non-zero exit preserves scriptability — `statuspulse status || alert`.
	// Applies in both human and JSON modes.
	if degraded {
		os.Exit(1)
	}
	return nil
}
