package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/wgillmer/statuspulse-cli/internal/client"
	"github.com/wgillmer/statuspulse-cli/internal/styles"
)

var incidentsCmd = &cobra.Command{
	Use:     "incidents",
	Aliases: []string{"incident", "i"},
	Short:   "Manage incidents on your status page",
}

func init() {
	rootCmd.AddCommand(incidentsCmd)
	incidentsCmd.AddCommand(incidentsListCmd)
	incidentsCmd.AddCommand(incidentsCreateCmd)
	incidentsCmd.AddCommand(incidentsGetCmd)
	incidentsCmd.AddCommand(incidentsUpdateCmd)
	incidentsCmd.AddCommand(incidentsPostCmd)
	incidentsCmd.AddCommand(incidentsResolveCmd)

	initIncidentsListFlags()
	initIncidentsCreateFlags()
	initIncidentsUpdateFlags()
	initIncidentsPostFlags()
	initIncidentsResolveFlags()
}

// ── list ──────────────────────────────────────────────────────────────

var listFlagStatus string

func initIncidentsListFlags() {
	incidentsListCmd.Flags().StringVar(&listFlagStatus, "status", "",
		"Filter by status: investigating, identified, monitoring, resolved")
}

var incidentsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List incidents for your org",
	RunE:    runIncidentsList,
}

func runIncidentsList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	incidents, err := c.ListIncidents(ctx, listFlagStatus)
	if err != nil {
		return handleAPIError(err, "incident", "")
	}
	if jsonOutput {
		return emit(incidents, nil)
	}
	if len(incidents) == 0 {
		fmt.Println(styles.Dim.Render("no incidents — ") + styles.Success.Render("clean slate."))
		return nil
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Header.Width(4).Render(""),
		styles.Header.Width(38).Render("ID"),
		styles.Header.Width(40).Render("TITLE"),
		styles.Header.Width(14).Render("STATUS"),
		styles.Header.Width(10).Render("IMPACT"),
		styles.Header.Width(18).Render("OPENED"),
	)
	fmt.Println(header)

	for _, i := range incidents {
		row := lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Cell.Width(4).Render(styles.StatusGlyph(i.Status)),
			styles.Cell.Width(38).Render(i.ID),
			styles.Cell.Width(40).Render(truncate(i.Title, 38)),
			styles.Cell.Width(14).Render(colorIncidentStatus(i.Status)),
			styles.Cell.Width(10).Render(colorImpact(i.Impact)),
			styles.Cell.Width(18).Render(relTime(i.CreatedAt)),
		)
		fmt.Println(row)
	}
	fmt.Println()
	fmt.Println(styles.Faint.Render(fmt.Sprintf("%d incident(s)", len(incidents))))
	return nil
}

// ── create ────────────────────────────────────────────────────────────

var (
	incCreateTitle      string
	incCreateImpact     string
	incCreateStatus     string
	incCreateMessage    string
	incCreateMonitorIDs []string
)

func initIncidentsCreateFlags() {
	f := incidentsCreateCmd.Flags()
	f.StringVar(&incCreateTitle, "title", "", "Short, user-facing summary")
	f.StringVar(&incCreateImpact, "impact", "", "Impact: none, minor, major, critical")
	f.StringVar(&incCreateStatus, "status", "", "Status: investigating, identified, monitoring, resolved")
	f.StringVar(&incCreateMessage, "message", "", "Initial update published on the status page")
	f.StringSliceVar(&incCreateMonitorIDs, "monitor", nil, "Affected monitor IDs (repeatable)")
}

var incidentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Open a new incident (interactive form or flag-driven)",
	RunE:  runIncidentsCreate,
}

func runIncidentsCreate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}

	interactive := incCreateTitle == ""
	if interactive {
		if err := requireFlagsForJSON("--title (and optionally --status, --impact, --message, --monitor)"); err != nil {
			return err
		}
	}

	title := incCreateTitle
	impact := firstNonEmpty(incCreateImpact, "minor")
	status := firstNonEmpty(incCreateStatus, "investigating")
	message := incCreateMessage

	if interactive {
		// Fetch monitors up front so the user can multi-select affected ones.
		ctx, cancel := signalCtx()
		defer cancel()
		monitors, _ := c.ListMonitors(ctx)

		opts := make([]huh.Option[string], 0, len(monitors))
		for _, m := range monitors {
			opts = append(opts, huh.NewOption(
				fmt.Sprintf("%s — %s", m.Name, m.Target), m.ID))
		}

		fields := []huh.Field{
			huh.NewInput().
				Title("Title").
				Description("Short, user-facing summary — appears at the top of the public status page.").
				Value(&title).
				Validate(requireNonEmpty("title")),
			huh.NewSelect[string]().
				Title("Impact").
				Options(
					huh.NewOption("None — informational", "none"),
					huh.NewOption("Minor — partial degradation", "minor"),
					huh.NewOption("Major — significant disruption", "major"),
					huh.NewOption("Critical — outage", "critical"),
				).
				Value(&impact),
			huh.NewSelect[string]().
				Title("Status").
				Options(
					huh.NewOption("Investigating", "investigating"),
					huh.NewOption("Identified", "identified"),
					huh.NewOption("Monitoring", "monitoring"),
					huh.NewOption("Resolved", "resolved"),
				).
				Value(&status),
			huh.NewText().
				Title("Initial update").
				Description("Shown to subscribers and published on the status page.").
				Lines(4).
				Value(&message),
		}
		if len(opts) > 0 {
			fields = append(fields, huh.NewMultiSelect[string]().
				Title("Affected monitors").
				Description("Select any monitors impacted by this incident.").
				Options(opts...).
				Value(&incCreateMonitorIDs))
		}

		form := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeCatppuccin())
		if err := form.Run(); err != nil {
			return err
		}
	}

	ctx, cancel := signalCtx()
	defer cancel()

	inc, err := c.CreateIncident(ctx, client.CreateIncidentRequest{
		Title:      strings.TrimSpace(title),
		Impact:     impact,
		Status:     status,
		Message:    strings.TrimSpace(message),
		MonitorIDs: incCreateMonitorIDs,
	})
	if err != nil {
		return handleAPIError(err, "incident", "")
	}
	return emitOK("incident", inc.ID, inc,
		styles.Check()+" incident opened "+styles.Faint.Render(inc.ID))
}

// ── get ───────────────────────────────────────────────────────────────

var incidentsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show an incident with full update timeline and affected monitors",
	Args:  cobra.ExactArgs(1),
	RunE:  runIncidentsGet,
}

func runIncidentsGet(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	d, err := c.GetIncident(ctx, args[0])
	if err != nil {
		return handleAPIError(err, "incident", args[0])
	}
	if jsonOutput {
		return emit(d, nil)
	}
	inc := d.Incident

	fmt.Println(styles.StatusGlyph(inc.Status) + " " + styles.Highlight.Render(inc.Title))
	fmt.Println(styles.Dim.Render("id       ") + inc.ID)
	fmt.Println(styles.Dim.Render("status   ") + colorIncidentStatus(inc.Status))
	fmt.Println(styles.Dim.Render("impact   ") + colorImpact(inc.Impact))
	fmt.Println(styles.Dim.Render("opened   ") + inc.CreatedAt.Local().Format("2006-01-02 15:04:05") +
		styles.Faint.Render("  ("+relTime(inc.CreatedAt)+")"))
	if inc.ResolvedAt != nil {
		fmt.Println(styles.Dim.Render("resolved ") + inc.ResolvedAt.Local().Format("2006-01-02 15:04:05"))
	}

	if len(d.Monitors) > 0 {
		fmt.Println()
		fmt.Println(styles.Accent.Render("affected monitors"))
		for _, m := range d.Monitors {
			fmt.Printf("  %s %s  %s  %s\n",
				styles.StatusGlyph(m.CurrentStatus),
				styles.Highlight.Render(m.Name),
				styles.Dim.Render(m.Type),
				styles.Faint.Render(m.ID))
		}
	}

	if len(d.Updates) > 0 {
		fmt.Println()
		fmt.Println(styles.Accent.Render("timeline"))
		for _, u := range d.Updates {
			fmt.Printf("  %s %s  %s\n",
				styles.StatusGlyph(u.Status),
				styles.Dim.Render(u.CreatedAt.Local().Format("2006-01-02 15:04:05")),
				colorIncidentStatus(u.Status),
			)
			fmt.Printf("    %s\n", u.Message)
		}
	}
	return nil
}

// ── update (status/impact) ────────────────────────────────────────────

var (
	incUpdateStatus string
	incUpdateImpact string
)

func initIncidentsUpdateFlags() {
	f := incidentsUpdateCmd.Flags()
	f.StringVar(&incUpdateStatus, "status", "", "New status: investigating, identified, monitoring, resolved")
	f.StringVar(&incUpdateImpact, "impact", "", "New impact: none, minor, major, critical")
}

var incidentsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Change an incident's status or impact (no timeline entry — use 'post' for that)",
	Args:  cobra.ExactArgs(1),
	RunE:  runIncidentsUpdate,
}

func runIncidentsUpdate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	hasFlags := cmd.Flags().Changed("status") || cmd.Flags().Changed("impact")
	status := incUpdateStatus
	impact := incUpdateImpact

	if !hasFlags {
		if err := requireFlagsForJSON("--status and/or --impact"); err != nil {
			return err
		}
		d, err := c.GetIncident(ctx, args[0])
		if err != nil {
			return handleAPIError(err, "incident", args[0])
		}
		status = d.Incident.Status
		impact = d.Incident.Impact

		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Status").
				Options(
					huh.NewOption("Investigating", "investigating"),
					huh.NewOption("Identified", "identified"),
					huh.NewOption("Monitoring", "monitoring"),
					huh.NewOption("Resolved", "resolved"),
				).
				Value(&status),
			huh.NewSelect[string]().
				Title("Impact").
				Options(
					huh.NewOption("None", "none"),
					huh.NewOption("Minor", "minor"),
					huh.NewOption("Major", "major"),
					huh.NewOption("Critical", "critical"),
				).
				Value(&impact),
		)).WithTheme(huh.ThemeCatppuccin())
		if err := form.Run(); err != nil {
			return err
		}
	}

	req := client.UpdateIncidentRequest{}
	if status != "" {
		req.Status = strp(status)
	}
	if impact != "" {
		req.Impact = strp(impact)
	}
	inc, err := c.UpdateIncident(ctx, args[0], req)
	if err != nil {
		return handleAPIError(err, "incident", args[0])
	}
	return emitOK("incident", inc.ID, inc,
		styles.Check()+" incident updated "+styles.Faint.Render(inc.ID))
}

// ── post (add timeline update) ────────────────────────────────────────

var (
	incPostStatus  string
	incPostMessage string
)

func initIncidentsPostFlags() {
	f := incidentsPostCmd.Flags()
	f.StringVar(&incPostStatus, "status", "", "Status for this update: investigating, identified, monitoring, resolved")
	f.StringVar(&incPostMessage, "message", "", "Update body (required)")
}

var incidentsPostCmd = &cobra.Command{
	Use:   "post <id>",
	Short: "Post a new timeline update on an incident",
	Args:  cobra.ExactArgs(1),
	RunE:  runIncidentsPost,
}

func runIncidentsPost(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}

	status := firstNonEmpty(incPostStatus, "investigating")
	message := incPostMessage

	if message == "" {
		if err := requireFlagsForJSON("--message (and optionally --status)"); err != nil {
			return err
		}
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Status").
				Options(
					huh.NewOption("Investigating", "investigating"),
					huh.NewOption("Identified", "identified"),
					huh.NewOption("Monitoring", "monitoring"),
					huh.NewOption("Resolved", "resolved"),
				).
				Value(&status),
			huh.NewText().
				Title("Update").
				Description("Published on the status page and to subscribers.").
				Lines(4).
				Value(&message).
				Validate(requireNonEmpty("message")),
		)).WithTheme(huh.ThemeCatppuccin())
		if err := form.Run(); err != nil {
			return err
		}
	}

	ctx, cancel := signalCtx()
	defer cancel()
	upd, err := c.AddIncidentUpdate(ctx, args[0], client.AddIncidentUpdateRequest{
		Status:  status,
		Message: strings.TrimSpace(message),
	})
	if err != nil {
		return handleAPIError(err, "incident", args[0])
	}
	return emitOK("update", upd.ID, upd,
		styles.Check()+" update posted "+styles.Faint.Render(upd.ID))
}

// ── resolve ───────────────────────────────────────────────────────────

var incResolveMessage string

func initIncidentsResolveFlags() {
	incidentsResolveCmd.Flags().StringVar(&incResolveMessage, "message", "",
		"Resolution message (required)")
}

var incidentsResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "Mark an incident as resolved with a closing message",
	Args:  cobra.ExactArgs(1),
	RunE:  runIncidentsResolve,
}

func runIncidentsResolve(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}

	message := incResolveMessage
	if message == "" {
		if err := requireFlagsForJSON("--message"); err != nil {
			return err
		}
		if err := huh.NewText().
			Title("Resolution message").
			Description("Published as the final update on the public status page.").
			Lines(3).
			Value(&message).
			Validate(requireNonEmpty("message")).
			WithTheme(huh.ThemeCatppuccin()).
			Run(); err != nil {
			return err
		}
	}

	ctx, cancel := signalCtx()
	defer cancel()

	upd, err := c.AddIncidentUpdate(ctx, args[0], client.AddIncidentUpdateRequest{
		Status:  "resolved",
		Message: strings.TrimSpace(message),
	})
	if err != nil {
		return handleAPIError(err, "incident", args[0])
	}
	return emitOK("update", upd.ID, upd, styles.Check()+" incident resolved")
}

// ── helpers ───────────────────────────────────────────────────────────

func colorIncidentStatus(s string) string {
	switch s {
	case "resolved":
		return styles.Success.Render(s)
	case "investigating", "identified":
		return styles.Warning.Render(s)
	case "monitoring":
		return styles.Info.Render(s)
	}
	return styles.Dim.Render(s)
}

func colorImpact(s string) string {
	switch s {
	case "critical", "major":
		return styles.Error.Render(s)
	case "minor":
		return styles.Warning.Render(s)
	case "none":
		return styles.Faint.Render(s)
	}
	return styles.Dim.Render(s)
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}
