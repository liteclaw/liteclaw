package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/liteclaw/liteclaw/internal/pairing"
	"github.com/spf13/cobra"
)

var knownPairingChannels = []string{"telegram", "discord", "imessage", "signal", "whatsapp", "slack"}

func NewPairingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pairing",
		Short: "Secure DM pairing (approve inbound requests)",
		Long:  `Manage pairing requests for secure Direct Message channels.`,
		Example: `  liteclaw pairing list telegram
  liteclaw pairing approve telegram 123456`,
	}

	cmd.AddCommand(newPairingListCommand())
	cmd.AddCommand(newPairingApproveCommand())

	return cmd
}

func newPairingListCommand() *cobra.Command {
	var jsonOutput bool
	var channelFlag string

	cmd := &cobra.Command{
		Use:   "list [channel]",
		Short: "List pending pairing requests",
		Example: `  liteclaw pairing list telegram
  liteclaw pairing list --channel discord --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channel := channelFlag
			if len(args) > 0 {
				channel = args[0]
			}

			if channel == "" {
				return fmt.Errorf("channel required. Use --channel <channel> or pass it as the first argument (expected one of: %s)", strings.Join(knownPairingChannels, ", "))
			}

			channel = normalizeChannel(channel)
			requests, err := pairing.ListChannelPairingRequests(channel)
			if err != nil {
				return fmt.Errorf("failed to list requests: %w", err)
			}

			if jsonOutput {
				type output struct {
					Channel  string                   `json:"channel"`
					Requests []pairing.PairingRequest `json:"requests"`
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(output{Channel: channel, Requests: requests})
			}

			if len(requests) == 0 {
				cmd.Printf("No pending %s pairing requests.\n", channel)
				return nil
			}

			cmd.Printf("Pairing requests (%d)\n", len(requests))

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			_, _ = fmt.Fprintln(w, "Code\tID\tMeta\tRequested")

			for _, r := range requests {
				meta := ""
				if len(r.Meta) > 0 {
					b, _ := json.Marshal(r.Meta)
					meta = string(b)
				}

				// Format time nicely
				t, err := time.Parse(time.RFC3339, r.CreatedAt)
				timeStr := r.CreatedAt
				if err == nil {
					timeStr = t.Format(time.Kitchen) // e.g. 3:04PM
					if time.Since(t) > 24*time.Hour {
						timeStr = t.Format("Jan 02")
					}
				}

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Code, r.ID, meta, timeStr)
			}
			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON output")
	cmd.Flags().StringVar(&channelFlag, "channel", "", fmt.Sprintf("Channel (%s)", strings.Join(knownPairingChannels, ", ")))

	return cmd
}

func newPairingApproveCommand() *cobra.Command {
	var notify bool
	var channelFlag string

	cmd := &cobra.Command{
		Use:   "approve <codeOrChannel> [code]",
		Short: "Approve a pairing code and allow that sender",
		Example: `  liteclaw pairing approve telegram 1234
  liteclaw pairing approve --channel discord 5678 --notify`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var code string
			var channel string

			// Argument parsing logic matching TS:
			// approve <channel> <code>
			// approve --channel <channel> <code>
			// approve <code> (implies channel must be guessed? No, TS logic: "Usage: ... --channel ...")

			if channelFlag != "" {
				channel = channelFlag
				if len(args) != 1 {
					return fmt.Errorf("too many arguments. Use: liteclaw pairing approve --channel <channel> <code>")
				}
				code = args[0]
			} else {
				// No channel flag, so first arg must be channel, second code
				if len(args) != 2 {
					return fmt.Errorf("usage: liteclaw pairing approve <channel> <code> (or use --channel flag)")
				}
				channel = args[0]
				code = args[1]
			}

			channel = normalizeChannel(channel)

			approved, err := pairing.ApproveChannelPairingCode(channel, code)
			if err != nil {
				return err
			}

			if approved == nil {
				return fmt.Errorf("no pending pairing request found for code: %s", code)
			}

			cmd.Printf("Approved %s sender %s.\n", channel, approved.ID)

			if notify {
				cmd.Println("Warning: --notify is not yet supported in LiteClaw CLI.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&notify, "notify", false, "Notify the requester (not supported yet)")
	cmd.Flags().StringVar(&channelFlag, "channel", "", fmt.Sprintf("Channel (%s)", strings.Join(knownPairingChannels, ", ")))

	return cmd
}

func normalizeChannel(ch string) string {
	ch = strings.ToLower(strings.TrimSpace(ch))
	// Basic validation/normalization if needed
	return ch
}
