package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/spf13/cobra"
)

func NewLogsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "View gateway logs (tail -f)",
		Long:  `View the real-time logs of the LiteClaw gateway server. Useful when running in background/detached mode.`,
		Example: `  # View logs (follows by default)
  liteclaw logs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine log file path
			logDir := filepath.Join(config.StateDir(), "logs")
			logFile := filepath.Join(logDir, "gateway.log")

			// Check existence
			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				return fmt.Errorf("log file not found at %s. Is the gateway running in detached mode?", logFile)
			}

			fmt.Printf("Displaying logs from: %s\n", logFile)
			fmt.Println("Press Ctrl+C to exit.")
			fmt.Println("---")

			// Check for 'tail' command availability
			tailPath, err := exec.LookPath("tail")
			if err != nil {
				return fmt.Errorf("'tail' command not found in PATH")
			}

			// Execute tail -f
			c := exec.Command(tailPath, "-f", logFile)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			// Handle interrupt nicely?
			// exec.Command.Run() will forward signals usually or user just kills it.
			return c.Run()
		},
	}
}
