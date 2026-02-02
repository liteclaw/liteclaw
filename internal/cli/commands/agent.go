package commands

import (
	"context"
	"fmt"

	"github.com/liteclaw/liteclaw/internal/agent"
	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/spf13/cobra"
)

func NewAgentCommand() *cobra.Command {
	var message string
	var sessionID string
	var local bool

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run an agent turn",
		Long:  `Run the agent locally to process a message.`,
		Example: `  # Run an agent locally with a specific message
  liteclaw agent --message "Tell me a joke"

  # Run with a specific session ID
  liteclaw agent --message "Continue conversation" --session "session-id" --local`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if message == "" && len(args) > 0 {
				message = args[0]
			}
			if message == "" {
				return fmt.Errorf("message required")
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Respect global verbose flag
			verbose, _ := cmd.Flags().GetBool("verbose")
			cfg.Logging.Verbose = verbose

			// Mock sender for CLI
			sender := &cliSender{}

			// Create Service
			svc := agent.NewService(cfg, sender)

			ctx := context.Background()

			// Process
			if verbose {
				fmt.Printf("Agent (%s) processing...\n", sessionID)
			}

			err = svc.ProcessChat(ctx, sessionID, message, func(delta string) {
				fmt.Print(delta)
			})
			fmt.Println() // Newline at end

			return err
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
	cmd.Flags().StringVar(&sessionID, "session", "main", "Session ID to use")
	cmd.Flags().BoolVar(&local, "local", false, "Run locally (embedded) instead of via Gateway")

	return cmd
}

// cliSender implements tools.MessageSender
type cliSender struct{}

func (s *cliSender) SendMessage(ctx context.Context, channel, to, text string) error {
	fmt.Printf("\n[Outbound -> %s:%s] %s\n", channel, to, text)
	return nil
}
