package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/gateway"
	"github.com/spf13/cobra"
)

func NewChannelsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Channel management",
		Long:  `Manage and view status of communication channels.`,
		Example: `  # Check status of all channels
  liteclaw channels status`,
	}

	cmd.AddCommand(newChannelsStatusCommand())
	// cmd.AddCommand(newChannelsLoginCommand()) // Future

	return cmd
}

func newChannelsStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show channel status",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				cmd.Printf("Error loading config: %v\n", err)
				return
			}

			// Try to hit Gateway API
			port := cfg.Gateway.Port
			if port == 0 {
				port = 18789 // Default
			}

			url := fmt.Sprintf("http://localhost:%d/api/status", port)
			client := &http.Client{Timeout: 1 * time.Second}
			req, _ := http.NewRequest("GET", url, nil)

			// Load token and add to request
			token, _ := gateway.LoadClawdbotToken()
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			resp, err := client.Do(req)

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

			if err == nil && resp.StatusCode == 200 {
				defer resp.Body.Close()
				// Parse Gateway Status
				type ChannelStatus struct {
					Name      string `json:"name"`
					Type      string `json:"type"`
					Connected bool   `json:"connected"`
					Sessions  int    `json:"sessions"`
				}
				type StatusResponse struct {
					Channels []ChannelStatus `json:"channels"`
					Status   string          `json:"status"`
					Uptime   string          `json:"uptime"`
				}

				var status StatusResponse
				if err := json.NewDecoder(resp.Body).Decode(&status); err == nil {
					cmd.Printf("Gateway: %s (Uptime: %s)\n\n", strings.ToUpper(status.Status), status.Uptime)
					fmt.Fprintln(w, "Channel\tType\tStatus\tSessions")
					for _, ch := range status.Channels {
						state := "Offline"
						if ch.Connected {
							state = "Online"
						}
						fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", ch.Name, ch.Type, state, ch.Sessions)
					}
					w.Flush()
					return
				}
			}

			// Fallback to static config
			cmd.Println("Gateway not reachable. Showing configured channels:")
			fmt.Fprintln(w, "Channel\tEnabled\tConfigured")

			// Check Config
			fmt.Fprintf(w, "telegram\t%v\t%v\n", cfg.Channels.Telegram.Enabled, cfg.Channels.Telegram.BotToken != "")
			fmt.Fprintf(w, "discord\t%v\t%v\n", cfg.Channels.Discord.Enabled, cfg.Channels.Discord.Token != "")
			fmt.Fprintf(w, "imessage\t%v\t%v\n", cfg.Channels.IMessage.Enabled, true) // dbPath optional?
			// fmt.Fprintf(w, "signal\t%v\t%v\n", cfg.Channels.Signal.Enabled, true)
			// fmt.Fprintf(w, "whatsapp\t%v\t%v\n", cfg.Channels.WhatsApp.Enabled, true)
			// fmt.Fprintf(w, "slack\t%v\t%v\n", cfg.Channels.Slack.Enabled, cfg.Channels.Slack.BotToken != "")
			fmt.Fprintf(w, "wecom\t%v\t%v\n", cfg.Channels.WeCom.Enabled, cfg.Channels.WeCom.Token != "")
			fmt.Fprintf(w, "qq\t%v\t%v\n", cfg.Channels.QQ.Enabled, cfg.Channels.QQ.AppID != 0)
			fmt.Fprintf(w, "feishu\t%v\t%v\n", cfg.Channels.Feishu.Enabled, cfg.Channels.Feishu.AppID != "")
			fmt.Fprintf(w, "dingtalk\t%v\t%v\n", cfg.Channels.DingTalk.Enabled, cfg.Channels.DingTalk.AppKey != "")
			// ... add others
			w.Flush()
		},
	}
}
