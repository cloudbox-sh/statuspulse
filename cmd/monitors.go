// Package cmd — monitors subcommand tree.
//
// Every subcommand supports both interactive (huh) and flag-driven invocation.
// If all required inputs are supplied via flags, the form is skipped so the
// command is scriptable and safe for AI agents / CI pipelines.
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/wgillmer/statuspulse-cli/internal/client"
	"github.com/wgillmer/statuspulse-cli/internal/styles"
)

var monitorsCmd = &cobra.Command{
	Use:     "monitors",
	Aliases: []string{"monitor", "m"},
	Short:   "Manage health-check monitors",
}

func init() {
	rootCmd.AddCommand(monitorsCmd)
	monitorsCmd.AddCommand(monitorsListCmd)
	monitorsCmd.AddCommand(monitorsCreateCmd)
	monitorsCmd.AddCommand(monitorsGetCmd)
	monitorsCmd.AddCommand(monitorsUpdateCmd)
	monitorsCmd.AddCommand(monitorsDeleteCmd)
	monitorsCmd.AddCommand(monitorsChecksCmd)
	monitorsCmd.AddCommand(monitorsUptimeCmd)

	initMonitorsCreateFlags()
	initMonitorsUpdateFlags()
	initMonitorsChecksFlags()
	initMonitorsUptimeFlags()
}

// ── list ──────────────────────────────────────────────────────────────

var monitorsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List monitors in your org",
	RunE:    runMonitorsList,
}

func runMonitorsList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	monitors, err := c.ListMonitors(ctx)
	if err != nil {
		return handleAPIError(err, "monitor", "")
	}
	return emit(monitors, func() {
		if len(monitors) == 0 {
			fmt.Println(styles.Dim.Render("no monitors yet — create one with ") +
				styles.Code.Render("statuspulse monitors create"))
			return
		}

		header := lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Header.Width(4).Render(""),
			styles.Header.Width(38).Render("ID"),
			styles.Header.Width(28).Render("NAME"),
			styles.Header.Width(7).Render("TYPE"),
			styles.Header.Width(36).Render("TARGET"),
			styles.Header.Width(10).Render("INTERVAL"),
			styles.Header.Width(10).Render("STATUS"),
		)
		fmt.Println(header)

		for _, m := range monitors {
			row := lipgloss.JoinHorizontal(lipgloss.Top,
				styles.Cell.Width(4).Render(styles.StatusGlyph(m.CurrentStatus)),
				styles.Cell.Width(38).Render(m.ID),
				styles.Cell.Width(28).Render(truncate(m.Name, 26)),
				styles.Cell.Width(7).Render(m.Type),
				styles.Cell.Width(36).Render(truncate(m.Target, 34)),
				styles.Cell.Width(10).Render(fmt.Sprintf("%ds", m.IntervalSeconds)),
				styles.Cell.Width(10).Render(colorStatus(m.CurrentStatus)),
			)
			fmt.Println(row)
		}
		fmt.Println()
		fmt.Println(styles.Faint.Render(fmt.Sprintf("%d monitor(s)", len(monitors))))
	})
}

// ── get ───────────────────────────────────────────────────────────────

var monitorsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a single monitor's details, recent checks, and linked incidents",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorsGet,
}

func runMonitorsGet(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	detail, err := c.GetMonitor(ctx, args[0])
	if err != nil {
		return handleAPIError(err, "monitor", args[0])
	}
	if jsonOutput {
		return emit(detail, nil)
	}

	m := detail.Monitor

	fmt.Println(styles.StatusGlyph(m.CurrentStatus) + " " + styles.Highlight.Render(m.Name))
	fmt.Println(styles.Dim.Render("id       ") + m.ID)
	fmt.Println(styles.Dim.Render("type     ") + m.Type)
	fmt.Println(styles.Dim.Render("target   ") + m.Target)
	fmt.Println(styles.Dim.Render("interval ") + fmt.Sprintf("%ds", m.IntervalSeconds))
	fmt.Println(styles.Dim.Render("timeout  ") + fmt.Sprintf("%ds", m.TimeoutSeconds))
	if m.Type == "http" {
		fmt.Println(styles.Dim.Render("method   ") + m.Method)
		if m.StatusRule != nil && *m.StatusRule != "" {
			fmt.Println(styles.Dim.Render("rule     ") + *m.StatusRule)
		} else {
			fmt.Println(styles.Dim.Render("expected ") + strconv.Itoa(m.ExpectedStatus))
		}
		if hdrs := formatHeaders(m.Headers); hdrs != "" {
			fmt.Println(styles.Dim.Render("headers  ") + hdrs)
		}
		if asserts := formatAssertions(m.BodyAssertions); asserts != "" {
			fmt.Println(styles.Dim.Render("body     ") + asserts)
		}
	}
	fmt.Println(styles.Dim.Render("status   ") + colorStatus(m.CurrentStatus))
	fmt.Println(styles.Dim.Render("enabled  ") + fmt.Sprintf("%t", m.Enabled))
	if m.LastCheckedAt != nil {
		fmt.Println(styles.Dim.Render("checked  ") + m.LastCheckedAt.Local().Format("2006-01-02 15:04:05"))
	}

	if len(detail.RecentChecks) > 0 {
		fmt.Println()
		fmt.Println(styles.Accent.Render("recent checks"))
		for _, r := range detail.RecentChecks {
			code := ""
			if r.StatusCode != nil {
				code = fmt.Sprintf(" [%d]", *r.StatusCode)
			}
			errStr := ""
			if r.Error != nil && *r.Error != "" {
				errStr = styles.Faint.Render("  " + truncate(*r.Error, 60))
			}
			fmt.Printf("  %s %s  %s  %dms%s%s\n",
				styles.StatusGlyph(r.Status),
				r.CheckedAt.Local().Format("2006-01-02 15:04:05"),
				colorStatus(r.Status),
				r.ResponseTimeMs,
				code,
				errStr,
			)
		}
	}
	if len(detail.Incidents) > 0 {
		fmt.Println()
		fmt.Println(styles.Accent.Render("recent incidents (90d)"))
		for _, i := range detail.Incidents {
			fmt.Printf("  %s %s  %s  %s\n",
				styles.StatusGlyph(i.Status),
				truncate(i.Title, 50),
				colorIncidentStatus(i.Status),
				relTime(i.CreatedAt),
			)
		}
	}
	return nil
}

// ── create ────────────────────────────────────────────────────────────

// Shared flag set for `monitors create`. Empty defaults (except --type) let
// us detect which flags the user actually set: if everything the form would
// ask for is present, we skip the TUI.
var (
	createFlagType           string
	createFlagName           string
	createFlagTarget         string
	createFlagInterval       int
	createFlagTimeout        int
	createFlagMethod         string
	createFlagExpectedStatus int
	createFlagStatusRule     string
	createFlagHeader         []string // "Key: Value", repeatable
	createFlagBodyAssertion  []string // "contains:foo" | "not_contains:foo" | "regex:^ok$", repeatable
	createFlagDisabled       bool
)

func initMonitorsCreateFlags() {
	f := monitorsCreateCmd.Flags()
	f.StringVar(&createFlagType, "type", "", "Monitor type: http, tcp, ping")
	f.StringVar(&createFlagName, "name", "", "Human-readable label")
	f.StringVar(&createFlagTarget, "target", "", "URL / host:port / hostname")
	f.IntVar(&createFlagInterval, "interval", 0, "Check interval in seconds (plan minimum applies)")
	f.IntVar(&createFlagTimeout, "timeout", 0, "Per-request timeout in seconds")
	f.StringVar(&createFlagMethod, "method", "", "HTTP method (GET, HEAD, POST) — http only")
	f.IntVar(&createFlagExpectedStatus, "expected-status", 0, "Expected HTTP status code — http only")
	f.StringVar(&createFlagStatusRule, "status-rule", "", "HTTP status rule, e.g. '2xx' or '200-204,301' — overrides --expected-status")
	f.StringArrayVar(&createFlagHeader, "header", nil, "Custom HTTP header 'Key: Value' (repeatable)")
	f.StringArrayVar(&createFlagBodyAssertion, "assert", nil, "Body assertion 'contains:ok' | 'not_contains:err' | 'regex:^ok$' (repeatable)")
	f.BoolVar(&createFlagDisabled, "disabled", false, "Create the monitor disabled (default: enabled)")
}

var monitorsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new monitor (interactive form or flag-driven)",
	Long: "Create a new monitor. With no flags, an interactive form walks you through\n" +
		"the options. Supply --type, --name, and --target to skip the form — handy\n" +
		"for scripts and AI agents.",
	RunE: runMonitorsCreate,
}

func runMonitorsCreate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}

	// Flag-driven mode: when --name and --target are supplied, skip the form
	// entirely. Everything else has sensible defaults already.
	interactive := createFlagName == "" || createFlagTarget == ""
	if interactive {
		if err := requireFlagsForJSON("--name, --target"); err != nil {
			return err
		}
	}

	mType := firstNonEmpty(createFlagType, "http")
	name := createFlagName
	target := createFlagTarget
	intervalStr := intOrDefault(createFlagInterval, "60")
	timeoutStr := intOrDefault(createFlagTimeout, "10")
	method := firstNonEmpty(createFlagMethod, "GET")
	expectedStatus := intOrDefault(createFlagExpectedStatus, "200")
	statusRule := createFlagStatusRule

	if interactive {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Monitor type").
					Options(
						huh.NewOption("HTTP — check a URL returns the expected status", "http"),
						huh.NewOption("TCP — check a host:port accepts connections", "tcp"),
						huh.NewOption("Ping — ICMP echo to a host", "ping"),
					).
					Value(&mType),
				huh.NewInput().
					Title("Name").
					Description("Human-readable label, e.g. 'API — eu-west'").
					Value(&name).
					Validate(requireNonEmpty("name")),
				huh.NewInput().
					Title("Target").
					DescriptionFunc(func() string { return targetHint(mType) }, &mType).
					Value(&target).
					Validate(func(s string) error {
						if err := requireNonEmpty("target")(s); err != nil {
							return err
						}
						return validateTarget(mType, s)
					}),
				huh.NewInput().
					Title("Interval (seconds)").
					Value(&intervalStr).
					Validate(validatePositiveInt("interval")),
				huh.NewInput().
					Title("Timeout (seconds)").
					Value(&timeoutStr).
					Validate(validatePositiveInt("timeout")),
			),
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("HTTP method").
					Options(
						huh.NewOption("GET", "GET"),
						huh.NewOption("HEAD", "HEAD"),
						huh.NewOption("POST", "POST"),
					).
					Value(&method),
				huh.NewInput().
					Title("Status rule (optional)").
					Description("Leave blank to use the expected status code. Examples: '2xx', '200-204,301'").
					Value(&statusRule),
				huh.NewInput().
					Title("Expected status code").
					Description("Ignored if a status rule is set.").
					Value(&expectedStatus).
					Validate(validatePositiveInt("expected status")),
			).WithHideFunc(func() bool { return mType != "http" }),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil {
			return err
		}
	}

	interval, _ := strconv.Atoi(intervalStr)
	timeout, _ := strconv.Atoi(timeoutStr)
	expected, _ := strconv.Atoi(expectedStatus)

	req := client.CreateMonitorRequest{
		Name:            strings.TrimSpace(name),
		Type:            mType,
		Target:          strings.TrimSpace(target),
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
	}
	if mType == "http" {
		req.Method = method
		req.ExpectedStatus = expected
		if s := strings.TrimSpace(statusRule); s != "" {
			req.StatusRule = &s
		}
		if hdrs, err := parseHeaderFlags(createFlagHeader); err != nil {
			return err
		} else if hdrs != nil {
			req.Headers = hdrs
		}
		if as, err := parseAssertionFlags(createFlagBodyAssertion); err != nil {
			return err
		} else if as != nil {
			req.BodyAssertions = as
		}
	}
	if createFlagDisabled {
		f := false
		req.Enabled = &f
	}

	ctx, cancel := signalCtx()
	defer cancel()

	m, err := c.CreateMonitor(ctx, req)
	if err != nil {
		return handleAPIError(err, "monitor", "")
	}
	return emitOK("monitor", m.ID, m,
		styles.Check()+" monitor created "+styles.Faint.Render(m.ID))
}

// ── update ────────────────────────────────────────────────────────────

var (
	updateFlagName           string
	updateFlagTarget         string
	updateFlagInterval       int
	updateFlagTimeout        int
	updateFlagMethod         string
	updateFlagExpectedStatus int
	updateFlagStatusRule     string
	updateFlagClearRule      bool
	updateFlagHeader         []string
	updateFlagClearHeaders   bool
	updateFlagAssertion      []string
	updateFlagClearAsserts   bool
	updateFlagEnable         bool
	updateFlagDisable        bool
)

func initMonitorsUpdateFlags() {
	f := monitorsUpdateCmd.Flags()
	f.StringVar(&updateFlagName, "name", "", "Rename the monitor")
	f.StringVar(&updateFlagTarget, "target", "", "Change the target URL/host")
	f.IntVar(&updateFlagInterval, "interval", 0, "Change the check interval (seconds)")
	f.IntVar(&updateFlagTimeout, "timeout", 0, "Change the request timeout (seconds)")
	f.StringVar(&updateFlagMethod, "method", "", "Change the HTTP method")
	f.IntVar(&updateFlagExpectedStatus, "expected-status", 0, "Change the expected HTTP status code")
	f.StringVar(&updateFlagStatusRule, "status-rule", "", "Set HTTP status rule (e.g. '2xx,301')")
	f.BoolVar(&updateFlagClearRule, "clear-status-rule", false, "Remove the status rule and fall back to --expected-status")
	f.StringArrayVar(&updateFlagHeader, "header", nil, "Replace headers; specify 'Key: Value' (repeatable). Use --clear-headers alone to remove all.")
	f.BoolVar(&updateFlagClearHeaders, "clear-headers", false, "Clear all custom headers")
	f.StringArrayVar(&updateFlagAssertion, "assert", nil, "Replace body assertions (repeatable). Use --clear-asserts alone to remove all.")
	f.BoolVar(&updateFlagClearAsserts, "clear-asserts", false, "Clear all body assertions")
	f.BoolVar(&updateFlagEnable, "enable", false, "Enable the monitor")
	f.BoolVar(&updateFlagDisable, "disable", false, "Disable the monitor")
}

var monitorsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a monitor's configuration (interactive when no flags are passed)",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorsUpdate,
}

func runMonitorsUpdate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	// If the user passed any mutation flag, skip the form and use them as-is.
	hasFlags := cmd.Flags().Changed("name") ||
		cmd.Flags().Changed("target") ||
		cmd.Flags().Changed("interval") ||
		cmd.Flags().Changed("timeout") ||
		cmd.Flags().Changed("method") ||
		cmd.Flags().Changed("expected-status") ||
		cmd.Flags().Changed("status-rule") || updateFlagClearRule ||
		cmd.Flags().Changed("header") || updateFlagClearHeaders ||
		cmd.Flags().Changed("assert") || updateFlagClearAsserts ||
		updateFlagEnable || updateFlagDisable

	var req client.UpdateMonitorRequest

	if !hasFlags {
		if err := requireFlagsForJSON("any of --name, --target, --interval, --timeout, --method, --expected-status, --status-rule, --clear-status-rule, --header, --clear-headers, --assert, --clear-asserts, --enable, --disable"); err != nil {
			return err
		}
		// Prefill the form with the current monitor state so nothing surprises.
		detail, err := c.GetMonitor(ctx, args[0])
		if err != nil {
			return handleAPIError(err, "monitor", args[0])
		}
		m := detail.Monitor

		name := m.Name
		target := m.Target
		intervalStr := strconv.Itoa(m.IntervalSeconds)
		timeoutStr := strconv.Itoa(m.TimeoutSeconds)
		method := m.Method
		expected := strconv.Itoa(m.ExpectedStatus)
		rule := ""
		if m.StatusRule != nil {
			rule = *m.StatusRule
		}
		enabled := m.Enabled

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Name").Value(&name).Validate(requireNonEmpty("name")),
				huh.NewInput().Title("Target").Value(&target).Validate(requireNonEmpty("target")),
				huh.NewInput().Title("Interval (seconds)").Value(&intervalStr).Validate(validatePositiveInt("interval")),
				huh.NewInput().Title("Timeout (seconds)").Value(&timeoutStr).Validate(validatePositiveInt("timeout")),
				huh.NewConfirm().Title("Enabled?").Value(&enabled),
			),
			huh.NewGroup(
				huh.NewSelect[string]().Title("HTTP method").
					Options(huh.NewOption("GET", "GET"), huh.NewOption("HEAD", "HEAD"), huh.NewOption("POST", "POST")).
					Value(&method),
				huh.NewInput().Title("Status rule").
					Description("Leave blank to use expected status. e.g. '2xx,301'").Value(&rule),
				huh.NewInput().Title("Expected status code").Value(&expected).Validate(validatePositiveInt("expected status")),
			).WithHideFunc(func() bool { return m.Type != "http" }),
		).WithTheme(huh.ThemeCatppuccin())
		if err := form.Run(); err != nil {
			return err
		}

		interval, _ := strconv.Atoi(intervalStr)
		timeout, _ := strconv.Atoi(timeoutStr)
		exp, _ := strconv.Atoi(expected)

		req = client.UpdateMonitorRequest{
			Name:            strp(name),
			Target:          strp(target),
			IntervalSeconds: &interval,
			TimeoutSeconds:  &timeout,
			Enabled:         &enabled,
		}
		if m.Type == "http" {
			req.Method = strp(method)
			req.ExpectedStatus = &exp
			trimmed := strings.TrimSpace(rule)
			if trimmed == "" {
				req.ClearStatusRule = true
			} else {
				req.StatusRule = strp(trimmed)
			}
		}
	} else {
		if updateFlagName != "" {
			req.Name = strp(updateFlagName)
		}
		if updateFlagTarget != "" {
			req.Target = strp(updateFlagTarget)
		}
		if cmd.Flags().Changed("interval") {
			req.IntervalSeconds = &updateFlagInterval
		}
		if cmd.Flags().Changed("timeout") {
			req.TimeoutSeconds = &updateFlagTimeout
		}
		if updateFlagMethod != "" {
			req.Method = strp(updateFlagMethod)
		}
		if cmd.Flags().Changed("expected-status") {
			req.ExpectedStatus = &updateFlagExpectedStatus
		}
		if updateFlagClearRule {
			req.ClearStatusRule = true
		} else if updateFlagStatusRule != "" {
			req.StatusRule = strp(updateFlagStatusRule)
		}
		if updateFlagClearHeaders {
			empty := json.RawMessage(`{}`)
			req.Headers = &empty
		} else if len(updateFlagHeader) > 0 {
			hdrs, err := parseHeaderFlags(updateFlagHeader)
			if err != nil {
				return err
			}
			req.Headers = &hdrs
		}
		if updateFlagClearAsserts {
			empty := json.RawMessage(`[]`)
			req.BodyAssertions = &empty
		} else if len(updateFlagAssertion) > 0 {
			as, err := parseAssertionFlags(updateFlagAssertion)
			if err != nil {
				return err
			}
			req.BodyAssertions = &as
		}
		if updateFlagEnable && updateFlagDisable {
			return fmt.Errorf("--enable and --disable are mutually exclusive")
		}
		if updateFlagEnable {
			t := true
			req.Enabled = &t
		}
		if updateFlagDisable {
			f := false
			req.Enabled = &f
		}
	}

	m, err := c.UpdateMonitor(ctx, args[0], req)
	if err != nil {
		return handleAPIError(err, "monitor", args[0])
	}
	return emitOK("monitor", m.ID, m,
		styles.Check()+" monitor updated "+styles.Faint.Render(m.ID))
}

// ── delete ────────────────────────────────────────────────────────────

var monitorsDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a monitor",
	Args:    cobra.ExactArgs(1),
	RunE:    runMonitorsDelete,
}

func runMonitorsDelete(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	// Scripted callers skip the confirmation via --json; interactive callers
	// get the usual huh prompt.
	if !jsonOutput {
		confirm := false
		if err := huh.NewConfirm().
			Title("Delete monitor " + args[0] + "?").
			Description("This cannot be undone.").
			Affirmative("Delete").
			Negative("Cancel").
			Value(&confirm).
			WithTheme(huh.ThemeCatppuccin()).
			Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(styles.Dim.Render("aborted"))
			return nil
		}
	}

	if err := c.DeleteMonitor(ctx, args[0]); err != nil {
		return handleAPIError(err, "monitor", args[0])
	}
	return emitOK("", args[0], nil, styles.Check()+" monitor deleted")
}

// ── checks ────────────────────────────────────────────────────────────

var (
	checksFlagLimit int
	checksFlagFrom  string
	checksFlagTo    string
)

func initMonitorsChecksFlags() {
	f := monitorsChecksCmd.Flags()
	f.IntVar(&checksFlagLimit, "limit", 100, "Maximum number of rows to fetch (1-1000)")
	f.StringVar(&checksFlagFrom, "from", "", "Window start, RFC3339 (e.g. 2026-04-19T00:00:00Z)")
	f.StringVar(&checksFlagTo, "to", "", "Window end, RFC3339 (defaults to now)")
}

var monitorsChecksCmd = &cobra.Command{
	Use:   "checks <id>",
	Short: "List recent check results for a monitor",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorsChecks,
}

func runMonitorsChecks(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	q := client.ChecksQuery{Limit: checksFlagLimit}
	if checksFlagFrom != "" {
		t, err := time.Parse(time.RFC3339, checksFlagFrom)
		if err != nil {
			return fmt.Errorf("--from: %w", err)
		}
		q.From = &t
	}
	if checksFlagTo != "" {
		t, err := time.Parse(time.RFC3339, checksFlagTo)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
		q.To = &t
	}

	checks, err := c.MonitorChecks(ctx, args[0], q)
	if err != nil {
		return handleAPIError(err, "monitor", args[0])
	}
	if jsonOutput {
		return emit(checks, nil)
	}
	if len(checks) == 0 {
		fmt.Println(styles.Dim.Render("no checks in range"))
		return nil
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Header.Width(4).Render(""),
		styles.Header.Width(20).Render("WHEN"),
		styles.Header.Width(10).Render("STATUS"),
		styles.Header.Width(8).Render("MS"),
		styles.Header.Width(6).Render("CODE"),
		styles.Header.Width(40).Render("ERROR"),
	)
	fmt.Println(header)
	for _, r := range checks {
		code := ""
		if r.StatusCode != nil {
			code = strconv.Itoa(*r.StatusCode)
		}
		errStr := ""
		if r.Error != nil {
			errStr = truncate(*r.Error, 38)
		}
		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Cell.Width(4).Render(styles.StatusGlyph(r.Status)),
			styles.Cell.Width(20).Render(r.CheckedAt.Local().Format("2006-01-02 15:04:05")),
			styles.Cell.Width(10).Render(colorStatus(r.Status)),
			styles.Cell.Width(8).Render(strconv.Itoa(r.ResponseTimeMs)),
			styles.Cell.Width(6).Render(code),
			styles.Cell.Width(40).Render(errStr),
		))
	}
	fmt.Println()
	fmt.Println(styles.Faint.Render(fmt.Sprintf("%d check(s)", len(checks))))
	return nil
}

// ── uptime ────────────────────────────────────────────────────────────

var uptimeFlagDays int

func initMonitorsUptimeFlags() {
	monitorsUptimeCmd.Flags().IntVar(&uptimeFlagDays, "days", 30, "Lookback window in days (1-365, plan cap applies)")
}

var monitorsUptimeCmd = &cobra.Command{
	Use:   "uptime <id>",
	Short: "Show uptime percentage and daily breakdown for a monitor",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorsUptime,
}

func runMonitorsUptime(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	rep, err := c.MonitorUptime(ctx, args[0], uptimeFlagDays)
	if err != nil {
		return handleAPIError(err, "monitor", args[0])
	}
	if jsonOutput {
		return emit(rep, nil)
	}
	fmt.Printf("%s uptime over %d days\n",
		styles.Highlight.Render(fmt.Sprintf("%.2f%%", rep.UptimePct)),
		rep.Days)
	fmt.Println()
	for _, d := range rep.Daily {
		fmt.Printf("  %s  %s%%  %s\n",
			styles.Dim.Render(d.Date),
			colorUptimePct(d.UptimePct),
			styles.Faint.Render(fmt.Sprintf("avg %.0fms", d.AvgResponseMs)),
		)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────

func requireNonEmpty(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

func validatePositiveInt(field string) func(string) error {
	return func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("%s must be a number", field)
		}
		if n <= 0 {
			return fmt.Errorf("%s must be > 0", field)
		}
		return nil
	}
}

func targetHint(mType string) string {
	switch mType {
	case "http":
		return "Full URL, e.g. https://api.example.com/health"
	case "tcp":
		return "host:port, e.g. db.example.com:5432"
	case "ping":
		return "Hostname or IP, e.g. example.com"
	}
	return ""
}

func validateTarget(mType, s string) error {
	s = strings.TrimSpace(s)
	switch mType {
	case "http":
		u, err := url.Parse(s)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("must be a full URL including scheme")
		}
	case "tcp":
		if !strings.Contains(s, ":") {
			return fmt.Errorf("must be host:port")
		}
	}
	return nil
}

func colorStatus(s string) string {
	switch s {
	case "up":
		return styles.Success.Render(s)
	case "down":
		return styles.Error.Render(s)
	case "degraded":
		return styles.Warning.Render(s)
	case "", "unknown":
		return styles.Faint.Render("pending")
	}
	return styles.Dim.Render(s)
}

func colorUptimePct(p float64) string {
	switch {
	case p >= 99.9:
		return styles.Success.Render(fmt.Sprintf("%6.2f", p))
	case p >= 95:
		return styles.Warning.Render(fmt.Sprintf("%6.2f", p))
	case p == 0:
		return styles.Faint.Render("   n/a")
	default:
		return styles.Error.Render(fmt.Sprintf("%6.2f", p))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func intOrDefault(n int, def string) string {
	if n > 0 {
		return strconv.Itoa(n)
	}
	return def
}

func strp(s string) *string { return &s }

// parseHeaderFlags turns ["Key: Value", "X-Foo: bar"] into the JSON object
// shape the API expects. Returns nil when no flags were supplied.
func parseHeaderFlags(flags []string) (json.RawMessage, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, h := range flags {
		idx := strings.Index(h, ":")
		if idx <= 0 {
			return nil, fmt.Errorf("--header %q: expected 'Key: Value'", h)
		}
		k := strings.TrimSpace(h[:idx])
		v := strings.TrimSpace(h[idx+1:])
		if k == "" {
			return nil, fmt.Errorf("--header %q: missing name", h)
		}
		out[k] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// parseAssertionFlags turns ["contains:ok", "regex:^OK$"] into the JSON array
// of {type, value} objects the API expects.
func parseAssertionFlags(flags []string) (json.RawMessage, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	type assertion struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	out := []assertion{}
	for _, a := range flags {
		idx := strings.Index(a, ":")
		if idx <= 0 {
			return nil, fmt.Errorf("--assert %q: expected 'type:value'", a)
		}
		t := strings.TrimSpace(a[:idx])
		v := a[idx+1:] // value may have colons; don't trim aggressively
		switch t {
		case "contains", "not_contains", "regex":
		default:
			return nil, fmt.Errorf("--assert %q: unknown type %q (use contains, not_contains, regex)", a, t)
		}
		out = append(out, assertion{Type: t, Value: v})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func formatHeaders(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return ""
	}
	parts := make([]string, 0, len(obj))
	for k, v := range obj {
		parts = append(parts, k+": "+v)
	}
	return strings.Join(parts, "  ")
}

func formatAssertions(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var list []struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
		return ""
	}
	parts := make([]string, 0, len(list))
	for _, a := range list {
		parts = append(parts, a.Type+":"+a.Value)
	}
	return strings.Join(parts, "  ")
}
