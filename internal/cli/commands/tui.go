package commands

import (
	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/internal/tui"
)

// NewTuiCommand creates the tui subcommand.
func NewTuiCommand() *cobra.Command {
	var host string
	var port int
	var token string

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Open a terminal UI connected to the Gateway",
		Long: `Open a terminal UI connected to the Gateway.

By default, connects to the Gateway using the port from config file (gateway.port).
Use --host and --port flags to connect to a different Gateway.`,
		Example: `  liteclaw tui                           # Connect using config file settings
  liteclaw tui --port 18789              # Connect to port 18789
  liteclaw tui --port 18789 --token <TOKEN> # Connect with auth token
  liteclaw tui --host 192.168.1.100      # Connect to remote Gateway`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := &tui.Config{
				Host:  host,
				Port:  port,
				Token: token,
			}
			return tui.RunWithConfig(cfg)
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Gateway host address (default: localhost)")
	cmd.Flags().IntVar(&port, "port", 0, "Gateway port (default: from config file, or 18789)")
	cmd.Flags().StringVar(&token, "token", "", "Gateway authentication token")

	return cmd
}
