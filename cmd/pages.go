package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/client"
	"github.com/cloudbox-sh/statuspulse/internal/styles"
)

var pagesCmd = &cobra.Command{
	Use:     "pages",
	Aliases: []string{"page", "p"},
	Short:   "Manage public status pages",
}

// Keep in sync with the allowedThemes map in api/status_pages.go.
var pageThemes = []string{"midnight", "parchment", "mono", "forest", "ocean", "rose", "custom", "light"}

// hexColorRe mirrors the API's regex so we fail locally before a round-trip.
var hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func init() {
	rootCmd.AddCommand(pagesCmd)
	pagesCmd.AddCommand(pagesListCmd)
	pagesCmd.AddCommand(pagesGetCmd)
	pagesCmd.AddCommand(pagesCreateCmd)
	pagesCmd.AddCommand(pagesUpdateCmd)
	pagesCmd.AddCommand(pagesDeleteCmd)
	pagesCmd.AddCommand(pagesAttachCmd)
	pagesCmd.AddCommand(pagesDetachCmd)

	initPagesGetFlags()
	initPagesCreateFlags()
	initPagesUpdateFlags()
	initPagesAttachFlags()
}

// ── list ──────────────────────────────────────────────────────────────

var pagesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List status pages in your org",
	RunE:    runPagesList,
}

func runPagesList(cmd *cobra.Command, args []string) error {
	c, resolved, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	pages, err := c.ListStatusPages(ctx)
	if err != nil {
		return handleAPIError(err, "status page", "")
	}
	if jsonOutput {
		return emit(pages, nil)
	}
	if len(pages) == 0 {
		fmt.Println(styles.Dim.Render("no status pages yet — create one with ") +
			styles.Code.Render("statuspulse pages create"))
		return nil
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Header.Width(38).Render("ID"),
		styles.Header.Width(24).Render("NAME"),
		styles.Header.Width(28).Render("ORG/SLUG"),
		styles.Header.Width(10).Render("MONITORS"),
		styles.Header.Width(10).Render("ENABLED"),
	)
	fmt.Println(header)

	for _, p := range pages {
		enabled := styles.Dim.Render("no")
		if p.Enabled {
			enabled = styles.Success.Render("yes")
		}
		// Full "<org>/<page>" address — this is what users need for
		// `statuspulse public status …` and for the public URL.
		address := p.OrgSlug + "/" + p.Slug
		row := lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Cell.Width(38).Render(p.ID),
			styles.Cell.Width(24).Render(truncate(p.Name, 22)),
			styles.Cell.Width(28).Render(truncate(address, 26)),
			styles.Cell.Width(10).Render(fmt.Sprintf("%d", len(p.MonitorIDs))),
			styles.Cell.Width(10).Render(enabled),
		)
		fmt.Println(row)
	}
	fmt.Println()
	fmt.Println(styles.Faint.Render(fmt.Sprintf("%d page(s) — public URL: %s/s/<org>/<slug>", len(pages), resolved.APIURL)))
	return nil
}

// ── get ───────────────────────────────────────────────────────────────

var pagesGetDays int

func initPagesGetFlags() {
	pagesGetCmd.Flags().IntVar(&pagesGetDays, "days", 0,
		"Lookback window in days for uptime calc (plan cap applies; 0 = server default)")
}

var pagesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a status page's full configuration, uptime, and recent incidents",
	Args:  cobra.ExactArgs(1),
	RunE:  runPagesGet,
}

func runPagesGet(cmd *cobra.Command, args []string) error {
	c, resolved, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	id, err := resolvePageID(ctx, c, args[0])
	if err != nil {
		return err
	}
	d, err := c.GetStatusPage(ctx, id, pagesGetDays)
	if err != nil {
		return handleAPIError(err, "status page", args[0])
	}
	if jsonOutput {
		return emit(d, nil)
	}
	p := d.Page

	fmt.Println(styles.Highlight.Render(p.Name) + "  " + styles.Faint.Render(resolved.APIURL+"/s/"+p.OrgSlug+"/"+p.Slug))
	fmt.Println(styles.Dim.Render("id            ") + p.ID)
	fmt.Println(styles.Dim.Render("slug          ") + p.OrgSlug + "/" + p.Slug)
	fmt.Println(styles.Dim.Render("theme         ") + p.Theme)
	if p.CustomDomain != nil {
		fmt.Println(styles.Dim.Render("custom domain ") + *p.CustomDomain)
	}
	if p.BrandColor != nil {
		fmt.Println(styles.Dim.Render("brand color   ") + *p.BrandColor)
	}
	if p.LogoURL != nil {
		fmt.Println(styles.Dim.Render("logo          ") + *p.LogoURL)
	}
	if p.FaviconURL != nil {
		fmt.Println(styles.Dim.Render("favicon       ") + *p.FaviconURL)
	}
	if p.SupportURL != nil {
		fmt.Println(styles.Dim.Render("support       ") + *p.SupportURL)
	}
	if p.Description != "" {
		fmt.Println(styles.Dim.Render("description   ") + p.Description)
	}
	if p.HeaderText != "" {
		fmt.Println(styles.Dim.Render("banner        ") + p.HeaderText)
	}
	fmt.Println(styles.Dim.Render("enabled       ") + fmt.Sprintf("%t", p.Enabled))
	fmt.Println(styles.Dim.Render("uptime (avg)  ") + fmt.Sprintf("%.2f%% over %d days", d.AggregateUptimePct, d.Days))

	if len(d.Monitors) > 0 {
		fmt.Println()
		fmt.Println(styles.Accent.Render("monitors"))
		for _, m := range d.Monitors {
			name := m.Name
			if m.DisplayName != nil && *m.DisplayName != "" {
				name = *m.DisplayName
			}
			fmt.Printf("  %s %-30s %s  %s\n",
				styles.StatusGlyph(m.CurrentStatus),
				truncate(name, 30),
				colorUptimePct(m.UptimePct),
				styles.Faint.Render(m.ID))
		}
	}

	if len(d.Incidents) > 0 {
		fmt.Println()
		fmt.Println(styles.Accent.Render("recent incidents"))
		for _, i := range d.Incidents {
			fmt.Printf("  %s %-50s %s  %s\n",
				styles.StatusGlyph(i.Status),
				truncate(i.Title, 50),
				colorImpact(i.Impact),
				relTime(i.CreatedAt))
		}
	}
	return nil
}

// ── create ────────────────────────────────────────────────────────────

var (
	pageCreateName        string
	pageCreateSlug        string
	pageCreateTheme       string
	pageCreateDescription string
	pageCreateBanner      string
	pageCreateBrandColor  string
	pageCreateLogoURL     string
	pageCreateFaviconURL  string
	pageCreateSupportURL  string
	pageCreateMonitorIDs  []string
)

func initPagesCreateFlags() {
	f := pagesCreateCmd.Flags()
	f.StringVar(&pageCreateName, "name", "", "Page name, e.g. 'Acme Status'")
	f.StringVar(&pageCreateSlug, "slug", "", "URL slug (per-org unique), e.g. 'prod' → /s/<org>/prod")
	f.StringVar(&pageCreateTheme, "theme", "", "Theme: midnight, parchment, mono, forest, ocean, rose, custom")
	f.StringVar(&pageCreateDescription, "description", "", "160-char tagline shown under the name")
	f.StringVar(&pageCreateBanner, "banner", "", "Banner message (e.g. scheduled maintenance notice)")
	f.StringVar(&pageCreateBrandColor, "brand-color", "", "Hex brand colour, e.g. '#cba6f7'")
	f.StringVar(&pageCreateLogoURL, "logo-url", "", "Logo image URL")
	f.StringVar(&pageCreateFaviconURL, "favicon-url", "", "Favicon URL")
	f.StringVar(&pageCreateSupportURL, "support-url", "", "Link shown as 'Support' in the header")
	f.StringSliceVar(&pageCreateMonitorIDs, "monitor", nil, "Monitor IDs to display on the page (repeatable)")
}

var pagesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new public status page (interactive form or flag-driven)",
	RunE:  runPagesCreate,
}

func runPagesCreate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	interactive := pageCreateName == "" || pageCreateSlug == ""
	if interactive {
		if err := requireFlagsForJSON("--name, --slug"); err != nil {
			return err
		}
	}

	name := pageCreateName
	slug := pageCreateSlug
	theme := firstNonEmpty(pageCreateTheme, "midnight")
	description := pageCreateDescription
	banner := pageCreateBanner
	brandColor := pageCreateBrandColor
	logoURL := pageCreateLogoURL
	faviconURL := pageCreateFaviconURL
	supportURL := pageCreateSupportURL

	if interactive {
		monitors, _ := c.ListMonitors(ctx)
		monitorOpts := make([]huh.Option[string], 0, len(monitors))
		for _, m := range monitors {
			monitorOpts = append(monitorOpts, huh.NewOption(
				fmt.Sprintf("%s — %s", m.Name, m.Target), m.ID))
		}

		themeOpts := make([]huh.Option[string], 0, len(pageThemes))
		for _, t := range pageThemes {
			themeOpts = append(themeOpts, huh.NewOption(t, t))
		}

		fields := []huh.Field{
			huh.NewInput().Title("Name").Value(&name).Validate(requireNonEmpty("name")),
			huh.NewInput().Title("Slug").
				Description("URL slug — will appear at /s/<org>/<slug>").
				Value(&slug).Validate(requireNonEmpty("slug")),
			huh.NewSelect[string]().Title("Theme").Options(themeOpts...).Value(&theme),
			huh.NewInput().Title("Description").
				Description("Short tagline (≤160 chars).").Value(&description),
			huh.NewInput().Title("Banner").
				Description("Optional message shown across the top (e.g. maintenance notice).").Value(&banner),
			huh.NewInput().Title("Brand colour").
				Description("Hex colour, e.g. '#cba6f7'. Leave blank to use theme default.").
				Value(&brandColor).Validate(validateOptionalHex),
			huh.NewInput().Title("Logo URL").Value(&logoURL),
			huh.NewInput().Title("Favicon URL").Value(&faviconURL),
			huh.NewInput().Title("Support URL").Value(&supportURL),
		}
		if len(monitorOpts) > 0 {
			fields = append(fields, huh.NewMultiSelect[string]().
				Title("Monitors").
				Description("Which monitors should this page display?").
				Options(monitorOpts...).
				Value(&pageCreateMonitorIDs))
		}
		if err := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
	} else if brandColor != "" {
		if err := validateOptionalHex(brandColor); err != nil {
			return err
		}
	}

	req := client.CreateStatusPageRequest{
		Name:        strings.TrimSpace(name),
		Slug:        strings.TrimSpace(slug),
		Theme:       theme,
		Description: description,
		HeaderText:  banner,
		MonitorIDs:  pageCreateMonitorIDs,
	}
	if brandColor != "" {
		req.BrandColor = strp(brandColor)
	}
	if logoURL != "" {
		req.LogoURL = strp(logoURL)
	}
	if faviconURL != "" {
		req.FaviconURL = strp(faviconURL)
	}
	if supportURL != "" {
		req.SupportURL = strp(supportURL)
	}

	p, err := c.CreateStatusPage(ctx, req)
	if err != nil {
		return handleAPIError(err, "status page", "")
	}
	return emitOK("status_page", p.ID, p,
		styles.Check()+" status page created "+styles.Faint.Render(p.ID))
}

// ── update ────────────────────────────────────────────────────────────

var (
	pageUpdateName        string
	pageUpdateSlug        string
	pageUpdateTheme       string
	pageUpdateDescription string
	pageUpdateBanner      string
	pageUpdateBrandColor  string
	pageUpdateLogoURL     string
	pageUpdateFaviconURL  string
	pageUpdateSupportURL  string
	pageUpdateCustomDom   string
	pageUpdateMonitorIDs  []string
	pageUpdateEnable      bool
	pageUpdateDisable     bool
	pageUpdateClearLogo   bool
	pageUpdateClearColor  bool
	pageUpdateClearFav    bool
	pageUpdateClearSup    bool
)

func initPagesUpdateFlags() {
	f := pagesUpdateCmd.Flags()
	f.StringVar(&pageUpdateName, "name", "", "Rename the page")
	f.StringVar(&pageUpdateSlug, "slug", "", "Change the URL slug")
	f.StringVar(&pageUpdateTheme, "theme", "", "Change the theme (midnight/parchment/mono/forest/ocean/rose/custom)")
	f.StringVar(&pageUpdateDescription, "description", "", "Change the tagline")
	f.StringVar(&pageUpdateBanner, "banner", "", "Change the banner message (empty string clears it)")
	f.StringVar(&pageUpdateBrandColor, "brand-color", "", "Change the brand colour (hex)")
	f.StringVar(&pageUpdateLogoURL, "logo-url", "", "Change the logo URL")
	f.StringVar(&pageUpdateFaviconURL, "favicon-url", "", "Change the favicon URL")
	f.StringVar(&pageUpdateSupportURL, "support-url", "", "Change the support link")
	f.StringVar(&pageUpdateCustomDom, "custom-domain", "", "Set a custom domain (CNAME target)")
	f.StringSliceVar(&pageUpdateMonitorIDs, "monitor", nil, "Replace monitor list with these IDs (repeatable). Use empty '' to clear.")
	f.BoolVar(&pageUpdateEnable, "enable", false, "Enable the page (make it publicly visible)")
	f.BoolVar(&pageUpdateDisable, "disable", false, "Disable the page")
	f.BoolVar(&pageUpdateClearLogo, "clear-logo", false, "Remove the logo")
	f.BoolVar(&pageUpdateClearColor, "clear-brand-color", false, "Remove the brand colour")
	f.BoolVar(&pageUpdateClearFav, "clear-favicon", false, "Remove the favicon")
	f.BoolVar(&pageUpdateClearSup, "clear-support-url", false, "Remove the support link")
}

var pagesUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a status page's configuration (interactive when no flags)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPagesUpdate,
}

func runPagesUpdate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	hasFlags := anyFlagChanged(cmd,
		"name", "slug", "theme", "description", "banner",
		"brand-color", "logo-url", "favicon-url", "support-url", "custom-domain",
		"monitor") ||
		pageUpdateEnable || pageUpdateDisable ||
		pageUpdateClearLogo || pageUpdateClearColor || pageUpdateClearFav || pageUpdateClearSup

	id, err := resolvePageID(ctx, c, args[0])
	if err != nil {
		return err
	}

	var req client.UpdateStatusPageRequest

	if !hasFlags {
		if err := requireFlagsForJSON("any of --name, --slug, --theme, --description, --banner, --brand-color, --logo-url, --favicon-url, --support-url, --custom-domain, --monitor, --enable, --disable, --clear-*"); err != nil {
			return err
		}
		d, err := c.GetStatusPage(ctx, id, 0)
		if err != nil {
			return handleAPIError(err, "status page", args[0])
		}
		p := d.Page

		monitors, _ := c.ListMonitors(ctx)
		monitorOpts := make([]huh.Option[string], 0, len(monitors))
		for _, m := range monitors {
			monitorOpts = append(monitorOpts, huh.NewOption(
				fmt.Sprintf("%s — %s", m.Name, m.Target), m.ID))
		}
		themeOpts := make([]huh.Option[string], 0, len(pageThemes))
		for _, t := range pageThemes {
			themeOpts = append(themeOpts, huh.NewOption(t, t))
		}

		name := p.Name
		slug := p.Slug
		theme := p.Theme
		description := p.Description
		banner := p.HeaderText
		brandColor := stringOrEmpty(p.BrandColor)
		logoURL := stringOrEmpty(p.LogoURL)
		faviconURL := stringOrEmpty(p.FaviconURL)
		supportURL := stringOrEmpty(p.SupportURL)
		customDomain := stringOrEmpty(p.CustomDomain)
		enabled := p.Enabled
		selectedMonitors := p.MonitorIDs

		fields := []huh.Field{
			huh.NewInput().Title("Name").Value(&name).Validate(requireNonEmpty("name")),
			huh.NewInput().Title("Slug").Value(&slug).Validate(requireNonEmpty("slug")),
			huh.NewSelect[string]().Title("Theme").Options(themeOpts...).Value(&theme),
			huh.NewInput().Title("Description").Value(&description),
			huh.NewInput().Title("Banner").Value(&banner),
			huh.NewInput().Title("Brand colour (hex, blank to clear)").Value(&brandColor).Validate(validateOptionalHex),
			huh.NewInput().Title("Logo URL (blank to clear)").Value(&logoURL),
			huh.NewInput().Title("Favicon URL (blank to clear)").Value(&faviconURL),
			huh.NewInput().Title("Support URL (blank to clear)").Value(&supportURL),
			huh.NewInput().Title("Custom domain").Value(&customDomain),
			huh.NewConfirm().Title("Enabled?").Value(&enabled),
		}
		if len(monitorOpts) > 0 {
			fields = append(fields, huh.NewMultiSelect[string]().
				Title("Monitors").Options(monitorOpts...).Value(&selectedMonitors))
		}
		if err := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}

		req.Name = strp(name)
		req.Slug = strp(slug)
		req.Theme = strp(theme)
		req.Description = strp(description)
		req.HeaderText = strp(banner)
		req.BrandColor = strp(brandColor)
		req.LogoURL = strp(logoURL)
		req.FaviconURL = strp(faviconURL)
		req.SupportURL = strp(supportURL)
		req.CustomDomain = strp(customDomain)
		req.Enabled = &enabled
		req.MonitorIDs = &selectedMonitors
	} else {
		if cmd.Flags().Changed("name") {
			req.Name = strp(pageUpdateName)
		}
		if cmd.Flags().Changed("slug") {
			req.Slug = strp(pageUpdateSlug)
		}
		if cmd.Flags().Changed("theme") {
			req.Theme = strp(pageUpdateTheme)
		}
		if cmd.Flags().Changed("description") {
			req.Description = strp(pageUpdateDescription)
		}
		if cmd.Flags().Changed("banner") {
			req.HeaderText = strp(pageUpdateBanner)
		}
		if cmd.Flags().Changed("custom-domain") {
			req.CustomDomain = strp(pageUpdateCustomDom)
		}
		if pageUpdateClearColor {
			req.BrandColor = strp("")
		} else if cmd.Flags().Changed("brand-color") {
			if err := validateOptionalHex(pageUpdateBrandColor); err != nil {
				return err
			}
			req.BrandColor = strp(pageUpdateBrandColor)
		}
		if pageUpdateClearLogo {
			req.LogoURL = strp("")
		} else if cmd.Flags().Changed("logo-url") {
			req.LogoURL = strp(pageUpdateLogoURL)
		}
		if pageUpdateClearFav {
			req.FaviconURL = strp("")
		} else if cmd.Flags().Changed("favicon-url") {
			req.FaviconURL = strp(pageUpdateFaviconURL)
		}
		if pageUpdateClearSup {
			req.SupportURL = strp("")
		} else if cmd.Flags().Changed("support-url") {
			req.SupportURL = strp(pageUpdateSupportURL)
		}
		if cmd.Flags().Changed("monitor") {
			ids := pageUpdateMonitorIDs
			req.MonitorIDs = &ids
		}
		if pageUpdateEnable && pageUpdateDisable {
			return fmt.Errorf("--enable and --disable are mutually exclusive")
		}
		if pageUpdateEnable {
			t := true
			req.Enabled = &t
		}
		if pageUpdateDisable {
			f := false
			req.Enabled = &f
		}
	}

	p, err := c.UpdateStatusPage(ctx, id, req)
	if err != nil {
		return handleAPIError(err, "status page", args[0])
	}
	return emitOK("status_page", p.ID, p,
		styles.Check()+" status page updated "+styles.Faint.Render(p.ID))
}

// ── delete ────────────────────────────────────────────────────────────

var pagesDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a status page",
	Args:    cobra.ExactArgs(1),
	RunE:    runPagesDelete,
}

func runPagesDelete(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	id, err := resolvePageID(ctx, c, args[0])
	if err != nil {
		return err
	}

	if !jsonOutput {
		confirm := false
		if err := huh.NewConfirm().
			Title("Delete status page " + args[0] + "?").
			Description("The public URL will stop responding. Monitors are not deleted.").
			Affirmative("Delete").Negative("Cancel").
			Value(&confirm).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(styles.Dim.Render("aborted"))
			return nil
		}
	}
	if err := c.DeleteStatusPage(ctx, id); err != nil {
		return handleAPIError(err, "status page", args[0])
	}
	return emitOK("", id, nil, styles.Check()+" status page deleted")
}

// ── attach / detach ───────────────────────────────────────────────────

var (
	attachDisplayName string
	attachSortOrder   int
)

func initPagesAttachFlags() {
	f := pagesAttachCmd.Flags()
	f.StringVar(&attachDisplayName, "display-name", "", "Override the monitor's name on this page")
	f.IntVar(&attachSortOrder, "sort-order", -1, "Position on the page (0-indexed; default: append)")
}

var pagesAttachCmd = &cobra.Command{
	Use:   "attach <page-id> <monitor-id>",
	Short: "Attach a monitor to a status page (idempotent — updates display name/order if already attached)",
	Args:  cobra.ExactArgs(2),
	RunE:  runPagesAttach,
}

func runPagesAttach(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	pageID, err := resolvePageID(ctx, c, args[0])
	if err != nil {
		return err
	}
	req := client.AttachMonitorRequest{MonitorID: args[1]}
	if attachDisplayName != "" {
		req.DisplayName = strp(attachDisplayName)
	}
	if cmd.Flags().Changed("sort-order") {
		so := attachSortOrder
		req.SortOrder = &so
	}
	att, err := c.AttachMonitor(ctx, pageID, req)
	if err != nil {
		return handleAPIError(err, "status page", args[0])
	}
	return emitOK("attachment", att.ID, att, styles.Check()+" monitor attached")
}

var pagesDetachCmd = &cobra.Command{
	Use:   "detach <page-id> <monitor-id>",
	Short: "Remove a monitor from a status page (monitor itself is not deleted)",
	Args:  cobra.ExactArgs(2),
	RunE:  runPagesDetach,
}

func runPagesDetach(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()
	pageID, err := resolvePageID(ctx, c, args[0])
	if err != nil {
		return err
	}
	if err := c.DetachMonitor(ctx, pageID, args[1]); err != nil {
		return handleAPIError(err, "status page", args[0])
	}
	return emitOK("", args[1], nil, styles.Check()+" monitor detached")
}

// ── helpers ───────────────────────────────────────────────────────────

// uuidRe loosely matches UUID v4 — it's fine to be permissive here, the
// API does the authoritative check. The point is "does this look like an
// ID vs. a slug?".
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// resolvePageID accepts either a page UUID or a slug. UUIDs pass through
// unchanged; slugs are resolved via a list call. The dashboard lists show
// both columns, so users reliably type one or the other.
func resolvePageID(ctx context.Context, c *client.Client, ref string) (string, error) {
	if uuidRe.MatchString(ref) {
		return ref, nil
	}
	pages, err := c.ListStatusPages(ctx)
	if err != nil {
		return "", handleAPIError(err)
	}
	for _, p := range pages {
		if p.Slug == ref {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("status page %q not found — run %s to see available slugs and IDs",
		ref, styles.Code.Render("statuspulse pages list"))
}

func validateOptionalHex(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !hexColorRe.MatchString(s) {
		return fmt.Errorf("must be a hex colour like #cba6f7")
	}
	return nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func anyFlagChanged(cmd *cobra.Command, names ...string) bool {
	for _, n := range names {
		if cmd.Flags().Changed(n) {
			return true
		}
	}
	return false
}
