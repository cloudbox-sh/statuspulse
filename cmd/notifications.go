package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/wgillmer/statuspulse-cli/internal/client"
	"github.com/wgillmer/statuspulse-cli/internal/styles"
)

var notificationsCmd = &cobra.Command{
	Use:     "notifications",
	Aliases: []string{"notification", "notifs", "n"},
	Short:   "Manage notification channels (email, webhook, …)",
}

func init() {
	rootCmd.AddCommand(notificationsCmd)
	notificationsCmd.AddCommand(notificationsListCmd)
	notificationsCmd.AddCommand(notificationsCreateCmd)
	notificationsCmd.AddCommand(notificationsUpdateCmd)
	notificationsCmd.AddCommand(notificationsDeleteCmd)

	initNotificationsCreateFlags()
	initNotificationsUpdateFlags()
}

// ── list ──────────────────────────────────────────────────────────────

var notificationsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List notification channels",
	RunE:    runNotificationsList,
}

func runNotificationsList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	channels, err := c.ListNotifications(ctx)
	if err != nil {
		return handleAPIError(err, "channel", "")
	}
	if jsonOutput {
		return emit(channels, nil)
	}
	if len(channels) == 0 {
		fmt.Println(styles.Dim.Render("no notification channels — add one with ") +
			styles.Code.Render("statuspulse notifications create"))
		return nil
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Header.Width(38).Render("ID"),
		styles.Header.Width(10).Render("TYPE"),
		styles.Header.Width(24).Render("NAME"),
		styles.Header.Width(10).Render("ENABLED"),
		styles.Header.Width(40).Render("CONFIG"),
	)
	fmt.Println(header)
	for _, ch := range channels {
		enabled := styles.Dim.Render("no")
		if ch.Enabled {
			enabled = styles.Success.Render("yes")
		}
		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Cell.Width(38).Render(ch.ID),
			styles.Cell.Width(10).Render(ch.Type),
			styles.Cell.Width(24).Render(truncate(ch.Name, 22)),
			styles.Cell.Width(10).Render(enabled),
			styles.Cell.Width(40).Render(truncate(summarizeConfig(ch.Type, ch.Config), 38)),
		))
	}
	fmt.Println()
	fmt.Println(styles.Faint.Render(fmt.Sprintf("%d channel(s)", len(channels))))
	return nil
}

// ── create ────────────────────────────────────────────────────────────

var (
	notifCreateType   string
	notifCreateName   string
	notifCreateEmail  string
	notifCreateURL    string
	notifCreateConfig string
)

func initNotificationsCreateFlags() {
	f := notificationsCreateCmd.Flags()
	f.StringVar(&notifCreateType, "type", "", "Channel type: email, webhook")
	f.StringVar(&notifCreateName, "name", "", "Display name")
	f.StringVar(&notifCreateEmail, "email", "", "Destination email (shorthand for --type email)")
	f.StringVar(&notifCreateURL, "url", "", "Webhook URL (shorthand for --type webhook)")
	f.StringVar(&notifCreateConfig, "config", "", "Raw JSON config (escape hatch for unusual channel types)")
}

var notificationsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a notification channel (interactive form or flag-driven)",
	Long: "Create a notification channel. Common cases:\n" +
		"  statuspulse notifications create --email ops@example.com --name 'Ops'\n" +
		"  statuspulse notifications create --url https://hooks.example.com/x --name 'Webhook'\n\n" +
		"For other channel types, pass the full JSON body with --type and --config.",
	RunE: runNotificationsCreate,
}

func runNotificationsCreate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}

	// Shorthand flags imply a type, so users don't always need --type.
	chType := notifCreateType
	if chType == "" {
		switch {
		case notifCreateEmail != "":
			chType = "email"
		case notifCreateURL != "":
			chType = "webhook"
		}
	}
	name := notifCreateName
	configJSON := notifCreateConfig

	interactive := chType == "" || name == "" || (configJSON == "" && notifCreateEmail == "" && notifCreateURL == "")
	if interactive {
		if err := requireFlagsForJSON("--type, --name, and either --email, --url, or --config"); err != nil {
			return err
		}
	}

	if interactive {
		chType = firstNonEmpty(chType, "email")
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().Title("Type").
					Options(
						huh.NewOption("Email", "email"),
						huh.NewOption("Webhook", "webhook"),
					).Value(&chType),
				huh.NewInput().Title("Name").
					Description("How this channel appears in the dashboard.").
					Value(&name).Validate(requireNonEmpty("name")),
			),
			huh.NewGroup(
				huh.NewInput().Title("Destination email").
					Value(&notifCreateEmail).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("email is required")
						}
						if !strings.Contains(s, "@") {
							return fmt.Errorf("not an email")
						}
						return nil
					}),
			).WithHideFunc(func() bool { return chType != "email" }),
			huh.NewGroup(
				huh.NewInput().Title("Webhook URL").
					Value(&notifCreateURL).
					Validate(requireNonEmpty("url")),
			).WithHideFunc(func() bool { return chType != "webhook" }),
		).WithTheme(huh.ThemeCatppuccin())
		if err := form.Run(); err != nil {
			return err
		}
	}

	config, err := buildChannelConfig(chType, notifCreateEmail, notifCreateURL, configJSON)
	if err != nil {
		return err
	}

	ctx, cancel := signalCtx()
	defer cancel()
	ch, err := c.CreateNotification(ctx, client.CreateNotificationRequest{
		Type:   chType,
		Name:   strings.TrimSpace(name),
		Config: config,
	})
	if err != nil {
		return handleAPIError(err, "channel", "")
	}
	return emitOK("channel", ch.ID, ch,
		styles.Check()+" channel created "+styles.Faint.Render(ch.ID))
}

// ── update ────────────────────────────────────────────────────────────

var (
	notifUpdateName   string
	notifUpdateEmail  string
	notifUpdateURL    string
	notifUpdateConfig string
	notifUpdateEnable bool
	notifUpdateDis    bool
)

func initNotificationsUpdateFlags() {
	f := notificationsUpdateCmd.Flags()
	f.StringVar(&notifUpdateName, "name", "", "Rename the channel")
	f.StringVar(&notifUpdateEmail, "email", "", "Change the destination email (email channels)")
	f.StringVar(&notifUpdateURL, "url", "", "Change the webhook URL (webhook channels)")
	f.StringVar(&notifUpdateConfig, "config", "", "Replace the raw JSON config")
	f.BoolVar(&notifUpdateEnable, "enable", false, "Enable the channel")
	f.BoolVar(&notifUpdateDis, "disable", false, "Disable the channel")
}

var notificationsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a notification channel",
	Args:  cobra.ExactArgs(1),
	RunE:  runNotificationsUpdate,
}

func runNotificationsUpdate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	// We need the channel's type to build a valid config from the shortcut
	// flags (--email / --url). One list call keeps the code simple; we can
	// switch to a dedicated GET endpoint if one ever ships.
	channels, err := c.ListNotifications(ctx)
	if err != nil {
		return handleAPIError(err, "channel", "")
	}
	var current *client.NotificationChannel
	for i, ch := range channels {
		if ch.ID == args[0] {
			current = &channels[i]
			break
		}
	}
	if current == nil {
		return fmt.Errorf("channel not found")
	}

	hasFlags := anyFlagChanged(cmd, "name", "email", "url", "config") ||
		notifUpdateEnable || notifUpdateDis

	var req client.UpdateNotificationRequest
	if !hasFlags {
		if err := requireFlagsForJSON("any of --name, --email, --url, --config, --enable, --disable"); err != nil {
			return err
		}
		// Interactive: only rename + toggle + edit the destination.
		name := current.Name
		enabled := current.Enabled
		email, urlStr := "", ""
		switch current.Type {
		case "email":
			var cfg struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(current.Config, &cfg)
			email = cfg.Email
		case "webhook":
			var cfg struct {
				URL string `json:"url"`
			}
			_ = json.Unmarshal(current.Config, &cfg)
			urlStr = cfg.URL
		}

		fields := []huh.Field{
			huh.NewInput().Title("Name").Value(&name).Validate(requireNonEmpty("name")),
			huh.NewConfirm().Title("Enabled?").Value(&enabled),
		}
		switch current.Type {
		case "email":
			fields = append(fields, huh.NewInput().Title("Destination email").Value(&email))
		case "webhook":
			fields = append(fields, huh.NewInput().Title("Webhook URL").Value(&urlStr))
		}
		if err := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
		req.Name = strp(name)
		req.Enabled = &enabled
		if current.Type == "email" && email != "" {
			cfg, err := buildChannelConfig("email", email, "", "")
			if err != nil {
				return err
			}
			req.Config = &cfg
		}
		if current.Type == "webhook" && urlStr != "" {
			cfg, err := buildChannelConfig("webhook", "", urlStr, "")
			if err != nil {
				return err
			}
			req.Config = &cfg
		}
	} else {
		if cmd.Flags().Changed("name") {
			req.Name = strp(notifUpdateName)
		}
		if notifUpdateEnable && notifUpdateDis {
			return fmt.Errorf("--enable and --disable are mutually exclusive")
		}
		if notifUpdateEnable {
			t := true
			req.Enabled = &t
		}
		if notifUpdateDis {
			f := false
			req.Enabled = &f
		}
		if notifUpdateConfig != "" {
			raw := json.RawMessage(notifUpdateConfig)
			if !json.Valid(raw) {
				return fmt.Errorf("--config is not valid JSON")
			}
			req.Config = &raw
		} else if notifUpdateEmail != "" || notifUpdateURL != "" {
			cfg, err := buildChannelConfig(current.Type, notifUpdateEmail, notifUpdateURL, "")
			if err != nil {
				return err
			}
			req.Config = &cfg
		}
	}

	ch, err := c.UpdateNotification(ctx, args[0], req)
	if err != nil {
		return handleAPIError(err, "channel", args[0])
	}
	return emitOK("channel", ch.ID, ch,
		styles.Check()+" channel updated "+styles.Faint.Render(ch.ID))
}

// ── delete ────────────────────────────────────────────────────────────

var notificationsDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a notification channel",
	Args:    cobra.ExactArgs(1),
	RunE:    runNotificationsDelete,
}

func runNotificationsDelete(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	if !jsonOutput {
		confirm := false
		if err := huh.NewConfirm().
			Title("Delete channel " + args[0] + "?").
			Description("Subscribers on this channel will stop receiving notifications.").
			Affirmative("Delete").Negative("Cancel").
			Value(&confirm).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(styles.Dim.Render("aborted"))
			return nil
		}
	}
	if err := c.DeleteNotification(ctx, args[0]); err != nil {
		return handleAPIError(err, "channel", args[0])
	}
	return emitOK("", args[0], nil, styles.Check()+" channel deleted")
}

// ── helpers ───────────────────────────────────────────────────────────

// buildChannelConfig prefers --config (raw JSON passthrough) over the
// shorthand --email / --url flags. It returns a valid JSON body for the API.
func buildChannelConfig(chType, email, urlStr, raw string) (json.RawMessage, error) {
	if raw != "" {
		if !json.Valid([]byte(raw)) {
			return nil, fmt.Errorf("--config is not valid JSON")
		}
		return json.RawMessage(raw), nil
	}
	switch chType {
	case "email":
		if email == "" {
			return nil, fmt.Errorf("--email is required for email channels")
		}
		b, _ := json.Marshal(map[string]string{"email": email})
		return b, nil
	case "webhook":
		if urlStr == "" {
			return nil, fmt.Errorf("--url is required for webhook channels")
		}
		b, _ := json.Marshal(map[string]string{"url": urlStr})
		return b, nil
	default:
		return nil, fmt.Errorf("unknown channel type %q — pass --config with a JSON body", chType)
	}
}

func summarizeConfig(chType string, raw json.RawMessage) string {
	switch chType {
	case "email":
		var cfg struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(raw, &cfg); err == nil && cfg.Email != "" {
			return cfg.Email
		}
	case "webhook":
		var cfg struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(raw, &cfg); err == nil && cfg.URL != "" {
			return cfg.URL
		}
	}
	return string(raw)
}
