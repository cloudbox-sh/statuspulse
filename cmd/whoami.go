package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wgillmer/statuspulse-cli/internal/styles"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the account behind the current session",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	c, resolved, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	me, err := c.Me(ctx)
	if err != nil {
		return handleAPIError(err)
	}

	return emit(map[string]any{
		"user":        me.User,
		"orgs":        me.Orgs,
		"plan_limits": me.PlanLimits,
		"api_url":     resolved.APIURL,
		"auth_source": resolved.Source,
	}, func() {
		fmt.Println(styles.Highlight.Render(me.User.Email) +
			styles.Dim.Render("  ("+me.User.Name+")"))
		fmt.Println(styles.Dim.Render("api  ") + resolved.APIURL)
		fmt.Println(styles.Dim.Render("auth ") + resolved.Source)
		if len(me.Orgs) > 0 {
			fmt.Println(styles.Dim.Render("org  ") + me.Orgs[0].Name +
				styles.Faint.Render(" ["+me.Orgs[0].Plan+"]"))
		}
		if me.PlanLimits != nil {
			pl := me.PlanLimits
			fmt.Println()
			fmt.Println(styles.Accent.Render("plan limits"))
			fmt.Printf("  %s %s\n", styles.Dim.Render("monitors     "), fmtLimit(pl.MaxMonitors))
			fmt.Printf("  %s %s\n", styles.Dim.Render("status pages "), fmtLimit(pl.MaxStatusPages))
			fmt.Printf("  %s %ds\n", styles.Dim.Render("min interval "), pl.MinIntervalSecs)
			fmt.Printf("  %s %d days\n", styles.Dim.Render("history      "), pl.HistoryDays)
		}
	})
}

func fmtLimit(n int) string {
	if n == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", n)
}
