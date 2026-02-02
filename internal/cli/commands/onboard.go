package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewOnboardCommand creates the onboarding wizard command.
func NewOnboardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Guided first-time setup",
		Long:  "Interactive onboarding wizard for first-time LiteClaw setup.",
		Example: `  # Run onboarding wizard
  liteclaw onboard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runConfigureWizard(cmd, "onboard"); err != nil {
				return fmt.Errorf("onboarding failed: %w", err)
			}
			return nil
		},
	}

	return cmd
}
