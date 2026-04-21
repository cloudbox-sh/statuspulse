package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/client"
	"github.com/cloudbox-sh/statuspulse/internal/styles"
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

	// In verbose mode fetch the per-entity lists so we can show names
	// (not just counts). One extra API call each — only under -v.
	var monitors []client.Monitor
	var openIncidents []client.Incident
	if verboseFlag {
		monitors, err = c.ListMonitors(ctx)
		if err != nil {
			return handleAPIError(err)
		}
		// The server's ?status= filter matches exact lifecycle values
		// (investigating | identified | monitoring | resolved). "open" is a
		// UX concept — any non-resolved status — so filter client-side.
		allIncidents, err := c.ListIncidents(ctx, "")
		if err != nil {
			return handleAPIError(err)
		}
		for _, inc := range allIncidents {
			if inc.Status != "resolved" {
				openIncidents = append(openIncidents, inc)
			}
		}
	}

	// JSON mode gets a wrapper shape under -v so callers can tell the
	// richer payload apart from the flat dashboard stats.
	payload := any(stats)
	if verboseFlag {
		payload = map[string]any{
			"stats":     stats,
			"monitors":  monitors,
			"incidents": openIncidents,
		}
	}

	// Under -v, group monitors by status so each count line can expand
	// inline with the names of the monitors it represents.
	var upMonitors, downMonitors []client.Monitor
	otherMonitors := map[string][]client.Monitor{}
	if verboseFlag {
		for _, m := range monitors {
			switch m.CurrentStatus {
			case "up":
				upMonitors = append(upMonitors, m)
			case "down":
				downMonitors = append(downMonitors, m)
			default:
				otherMonitors[m.CurrentStatus] = append(otherMonitors[m.CurrentStatus], m)
			}
		}
	}

	// ID first so `statuspulse monitors get <id>` is one cursor-drag away;
	// dim-render so it's visually secondary to the human-readable name.
	printMonitors := func(ms []client.Monitor) {
		for _, m := range ms {
			fmt.Printf("      %s %s  %-24s %s\n",
				styles.StatusGlyph(m.CurrentStatus),
				styles.Dim.Render(m.ID),
				m.Name,
				styles.Dim.Render(m.Target))
		}
	}

	if err := emit(payload, func() {
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
		if verboseFlag {
			printMonitors(upMonitors)
		}

		fmt.Printf("  %s %s down\n",
			styles.StatusGlyph("down"), styles.Error.Render(fmt.Sprintf("%d", stats.DownCount)))
		if verboseFlag {
			printMonitors(downMonitors)
		}

		// Anything the dashboard stats don't capture as up/down (e.g.
		// degraded, unknown) only surfaces under -v, to avoid drift with
		// the server's count summary.
		if verboseFlag {
			for status, ms := range otherMonitors {
				fmt.Printf("  %s %s %s\n",
					styles.StatusGlyph(status),
					styles.Warning.Render(fmt.Sprintf("%d", len(ms))),
					status)
				printMonitors(ms)
			}
		}

		fmt.Printf("  %s %s active incidents\n",
			styles.StatusGlyph("investigating"), styles.Warning.Render(fmt.Sprintf("%d", stats.IncidentsActive)))
		if verboseFlag {
			for _, inc := range openIncidents {
				fmt.Printf("      %s %s  %-28s %s\n",
					styles.Warning.Render("●"),
					styles.Dim.Render(inc.ID),
					inc.Title,
					styles.Dim.Render(inc.Status+" · "+inc.Impact))
			}
		}

		// Hint at drill-down commands when there's something to drill into.
		if verboseFlag && (len(monitors) > 0 || len(openIncidents) > 0) {
			fmt.Println()
			fmt.Println(styles.Dim.Render("  drill down: statuspulse monitors get <id>  ·  statuspulse incidents get <id>"))
		}
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
