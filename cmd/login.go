package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/config"
	"github.com/cloudbox-sh/statuspulse/internal/styles"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate against the StatusPulse API",
	Long:  "Prompts for email and password and stores the session token at ~/.config/cloudbox/statuspulse.json.",
	RunE:  runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	c, resolved, err := newAnonClient()
	if err != nil {
		return err
	}

	// Login requires an interactive TUI (password prompt). --json mode is
	// for scripts, which should authenticate with STATUSPULSE_API_KEY instead.
	if jsonOutput {
		return fmt.Errorf("--json mode does not support interactive login — set STATUSPULSE_API_KEY instead")
	}

	fmt.Println(styles.Accent.Render("→ statuspulse login") +
		styles.Dim.Render(" ("+resolved.APIURL+")"))

	var email, password string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Email").
				Value(&email).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("email is required")
					}
					if !strings.Contains(s, "@") {
						return fmt.Errorf("that doesn't look like an email")
					}
					return nil
				}),
			huh.NewInput().
				Title("Password").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("password is required")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCatppuccin())

	if err := form.Run(); err != nil {
		return err
	}

	ctx, cancel := signalCtx()
	defer cancel()

	resp, err := c.Login(ctx, strings.TrimSpace(email), password)
	if err != nil {
		return err
	}

	cfg := &config.Config{
		APIURL:    resolved.APIURL,
		Token:     resp.Token,
		UserEmail: resp.User.Email,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	path, _ := config.Path()
	fmt.Fprintln(os.Stderr, styles.Check()+" logged in as "+
		styles.Highlight.Render(resp.User.Email)+
		styles.Dim.Render(" (token stored at "+path+")"))
	return nil
}
