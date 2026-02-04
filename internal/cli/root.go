// Package cli provides the command-line interface for LiteClaw.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/internal/cli/commands"
	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/version"
)

var rootCmd = &cobra.Command{
	Use:   "liteclaw",
	Short: "LiteClaw - Lightweight Personal AI Assistant",
	Long: `LiteClaw is a lightweight Go rewrite of Clawdbot.
It provides a personal AI assistant that runs on your own devices
with minimal resource usage.`,
	Version: version.Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if shouldSkipSelfCheck(cmd) {
			return nil
		}
		return ensureFirstRunReady(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		// When user runs `liteclaw` without subcommand
		if !isConfigured() {
			showFirstRunMenu(cmd)
		} else {
			_ = cmd.Help()
		}
	},
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(commands.NewGatewayCommand())
	rootCmd.AddCommand(commands.NewConfigureCommand())
	rootCmd.AddCommand(commands.NewOnboardCommand())
	rootCmd.AddCommand(commands.NewStatusCommand())
	rootCmd.AddCommand(commands.NewVersionCommand())
	rootCmd.AddCommand(commands.NewTuiCommand())
	rootCmd.AddCommand(commands.NewMCPCommand())
	rootCmd.AddCommand(commands.NewModelsCommand())
	rootCmd.AddCommand(commands.NewSkillCommand())
	rootCmd.AddCommand(commands.NewPairingCommand())
	rootCmd.AddCommand(commands.NewMessageCommand())
	rootCmd.AddCommand(commands.NewSessionsCommand())
	rootCmd.AddCommand(commands.NewChannelsCommand())
	rootCmd.AddCommand(commands.NewConfigCommand())
	rootCmd.AddCommand(commands.NewAgentCommand())
	rootCmd.AddCommand(commands.NewLogsCommand())
	rootCmd.AddCommand(commands.NewCronCommand())

	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is ~/.liteclaw/liteclaw.json)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output")
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func shouldSkipSelfCheck(cmd *cobra.Command) bool {
	name := cmd.Name()
	// Skip self-check for these commands and when running root without subcommand
	if name == "liteclaw" || name == "onboard" || name == "help" || name == "completion" || name == "version" {
		return true
	}
	return false
}

func isConfigured() bool {
	stateDir := config.StateDir()
	if _, err := os.Stat(stateDir); err != nil {
		return false
	}
	cfgPath := config.ConfigPath()
	if _, err := os.Stat(cfgPath); err != nil {
		return false
	}
	return true
}

func showFirstRunMenu(cmd *cobra.Command) {
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "🦞 Welcome to LiteClaw!")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintf(out, "   ~/.liteclaw folder not found. This appears to be your first run.\n")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "   What would you like to do?")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "   [1] 🚀 Start onboarding wizard (recommended for first-time setup)")
	_, _ = fmt.Fprintln(out, "   [2] 📖 View help information")
	_, _ = fmt.Fprintln(out, "   [3] 🚪 Exit")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprint(out, "   Enter your choice (1/2/3): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(input)

	_, _ = fmt.Fprintln(out, "")

	switch choice {
	case "1":
		// Run onboard command
		onboardCmd := commands.NewOnboardCommand()
		onboardCmd.SetOut(out)
		onboardCmd.SetIn(os.Stdin)
		_ = onboardCmd.Execute()
	case "2":
		_ = cmd.Help()
	case "3", "":
		_, _ = fmt.Fprintln(out, "   Goodbye! Run 'liteclaw onboard' when you're ready to set up.")
	default:
		_, _ = fmt.Fprintf(out, "   Invalid choice '%s'. Run 'liteclaw onboard' to get started.\n", choice)
	}
}

func ensureFirstRunReady(cmd *cobra.Command) error {
	if !isConfigured() {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "❌ LiteClaw is not configured yet.")
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "   Run: liteclaw onboard")
		return fmt.Errorf("not configured")
	}

	if err := config.EnsureExtrasFile(); err != nil {
		return err
	}

	return nil
}
