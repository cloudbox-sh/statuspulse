package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/statuspulse/internal/styles"
)

var completionStdout bool

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Install or print a shell completion script",
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.ExactArgs(1),
	Long: "Install a shell completion script for statuspulse.\n\n" +
		"By default the script is written to your shell's standard completion\n" +
		"directory and this command prints the one-line snippet needed to\n" +
		"activate it. Pass --stdout to print the script to stdout instead\n" +
		"(useful when you want to pipe it into a non-default location).\n\n" +
		"Examples:\n" +
		"  statuspulse completion zsh                                # auto-install + instructions\n" +
		"  statuspulse completion bash                               # auto-install + instructions\n" +
		"  statuspulse completion bash --stdout > /etc/bash_completion.d/statuspulse\n" +
		"  statuspulse completion powershell --stdout >> $PROFILE    # Windows/PowerShell",
	RunE: runCompletion,
}

func init() {
	// Replace cobra's auto-generated `completion` command with ours.
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	completionCmd.Flags().BoolVar(&completionStdout, "stdout", false,
		"Print the completion script to stdout instead of writing a file")
	rootCmd.AddCommand(completionCmd)
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]

	// --stdout: pipeline mode, preserve old behaviour for scripts that
	// pipe the script to a custom path.
	if completionStdout {
		return writeCompletionScript(shell, os.Stdout)
	}

	// PowerShell doesn't have a conventional per-user completion directory
	// the way zsh/bash/fish do — the user appends into $PROFILE themselves.
	if shell == "powershell" {
		return fmt.Errorf("powershell auto-install is not supported — use:\n" +
			"  statuspulse completion powershell --stdout >> $PROFILE")
	}

	path, enable, err := completionTarget(shell)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create completion directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write completion script: %w", err)
	}
	defer f.Close()

	if err := writeCompletionScript(shell, f); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, styles.Check()+" installed "+shell+" completion at "+
		styles.Highlight.Render(path))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, styles.Dim.Render("To enable:"))
	fmt.Fprintln(os.Stderr, enable)
	return nil
}

// completionTarget returns the conventional per-user install path for the
// given shell plus a short block of instructions to activate it. The paths
// chosen are the ones that don't require sudo and that most users will
// recognise.
func completionTarget(shell string) (path, enable string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory: %w", err)
	}
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zsh", "completions", "_statuspulse"),
			"  Add to your ~/.zshrc (once), then open a new shell:\n" +
				"    fpath=(~/.zsh/completions $fpath)\n" +
				"    autoload -U compinit && compinit", nil
	case "bash":
		return filepath.Join(home, ".local", "share", "bash-completion", "completions", "statuspulse"),
			"  Already on bash-completion's XDG search path.\n" +
				"  Open a new shell, or source the file directly:\n" +
				"    source " + filepath.Join(home, ".local", "share", "bash-completion", "completions", "statuspulse"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "completions", "statuspulse.fish"),
			"  Already on fish's completion path. Open a new shell to pick it up.", nil
	}
	return "", "", fmt.Errorf("unsupported shell: %s", shell)
}

func writeCompletionScript(shell string, w io.Writer) error {
	switch shell {
	case "bash":
		return rootCmd.GenBashCompletionV2(w, true)
	case "zsh":
		return rootCmd.GenZshCompletion(w)
	case "fish":
		return rootCmd.GenFishCompletion(w, true)
	case "powershell":
		return rootCmd.GenPowerShellCompletionWithDesc(w)
	}
	return fmt.Errorf("unsupported shell: %s", shell)
}
