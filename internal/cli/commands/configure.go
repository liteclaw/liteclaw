// Package commands provides CLI subcommands for LiteClaw.
package commands

import (
	"github.com/spf13/cobra"
)

// NewConfigureCommand creates the configure subcommand.
func NewConfigureCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Configure LiteClaw settings",
		Long:  `Interactive configuration wizard for LiteClaw settings.`,
		Example: `  # Run interactive wizard
  liteclaw configure

  # Configure specific section
  liteclaw configure --section telegram`,
		Run: func(cmd *cobra.Command, args []string) {
			runConfigure(cmd, args)
		},
	}

	cmd.Flags().String("section", "", "Configure specific section (telegram, discord, llm, etc.)")

	return cmd
}

func runConfigure(cmd *cobra.Command, args []string) {
	section, _ := cmd.Flags().GetString("section")

	if section != "" {
		cmd.Printf("Configuring section: %s\n", section)
		// TODO: Implement section-specific configuration
	} else {
		cmd.Println("LiteClaw Configuration")
		cmd.Println("======================")
		cmd.Println("This command is reserved for advanced configuration.")
		cmd.Println("For first-time setup, run: liteclaw onboard")
	}
}
