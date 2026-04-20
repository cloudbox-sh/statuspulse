package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wgillmer/statuspulse-cli/internal/config"
	"github.com/wgillmer/statuspulse-cli/internal/styles"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Forget the stored session token",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	// Best-effort: if the token is stale the server call will fail, but we
	// still want to wipe the local file so `whoami` reflects reality.
	if c, _, err := newClient(); err == nil {
		ctx, cancel := signalCtx()
		defer cancel()
		_ = c.Logout(ctx)
	}
	if err := config.Clear(); err != nil {
		return fmt.Errorf("clear config: %w", err)
	}
	if jsonOutput {
		return emit(map[string]any{"ok": true}, nil)
	}
	fmt.Fprintln(os.Stderr, styles.Check()+" logged out")
	return nil
}
