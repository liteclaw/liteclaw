// Package commands provides CLI subcommands for LiteClaw.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/internal/version"
)

// NewVersionCommand creates the version subcommand.
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the version number",
		Example: `  liteclaw version`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("LiteClaw %s\n", version.Version)
			cmd.Printf("  Commit: %s\n", version.Commit)
			cmd.Printf("  Built:  %s\n", version.BuildDate)
		},
	}
}
