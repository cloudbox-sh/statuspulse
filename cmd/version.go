package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/styles"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the statuspulse version",
	Long: "Prints the version of this statuspulse binary.\n\n" +
		"Set at release time by GoReleaser via -ldflags. For builds made directly\n" +
		"from source (go build / go install) the version is derived from Go's\n" +
		"embedded build info — either the module tag (`go install @v0.0.4`) or\n" +
		"`dev+<commit>[-dirty]` for a plain checkout.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return emit(map[string]string{"version": Version}, func() {
			fmt.Println(styles.Accent.Render("statuspulse") + " " + Version)
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
