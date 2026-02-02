package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/extensions/discord"
	"github.com/liteclaw/liteclaw/extensions/telegram"
	"github.com/liteclaw/liteclaw/internal/channels"
	"github.com/liteclaw/liteclaw/internal/config"
)

func NewMessageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Send messages and channel actions",
		Long:  `Send messages directly via configured channels without running the full gateway.`,
	}

	cmd.AddCommand(newMessageSendCommand())
	// Future: reaction, poll, etc.

	return cmd
}

func newMessageSendCommand() *cobra.Command {
	var target string
	var message string
	var channelFlag string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message",
		Example: `  liteclaw message send --channel telegram --target 12345 --message "Hello"
  liteclaw message send --channel discord --target 98765 --message "Hi" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return fmt.Errorf("target is required")
			}
			if message == "" {
				return fmt.Errorf("message is required")
			}
			if channelFlag == "" {
				return fmt.Errorf("channel is required")
			}

			if err := runMessageSend(channelFlag, target, message); err != nil {
				return fmt.Errorf("send failed: %w", err)
			}

			if jsonOutput {
				cmd.Println(`{"success": true}`)
			} else {
				cmd.Println("Message sent.")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "Target (chat ID, user handle)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message text")
	cmd.Flags().StringVar(&channelFlag, "channel", "", "Channel to use (telegram, discord)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output result as JSON")

	return cmd
}

func runMessageSend(channelName, target, text string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load error: %w", err)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	ctx := context.Background()

	var adapter channels.Adapter

	switch channelName {
	case "telegram":
		if !cfg.Channels.Telegram.Enabled {
			return fmt.Errorf("telegram not enabled in config")
		}
		tgCfg := &telegram.Config{
			Token: cfg.Channels.Telegram.BotToken,
		}
		tgAdapter := telegram.New(tgCfg, logger)

		// InitClient avoids starting poller
		if err := tgAdapter.InitClient(ctx); err != nil {
			return err
		}
		// We also need to stop? Stop mainly stops poller/connection.
		// For http client it doesn't matter much.
		adapter = tgAdapter

	case "discord":
		if !cfg.Channels.Discord.Enabled {
			return fmt.Errorf("discord not enabled")
		}
		discCfg := &discord.Config{
			Token: cfg.Channels.Discord.Token,
		}
		discAdapter := discord.New(discCfg, logger)

		// InitClient avoids starting WS
		if err := discAdapter.InitClient(ctx); err != nil {
			return err
		}
		adapter = discAdapter

	default:
		return fmt.Errorf("channel '%s' not supported for CLI sending yet", channelName)
	}

	_, err = adapter.Send(ctx, &channels.SendRequest{
		To:   channels.Destination{ChatID: target},
		Text: text,
	})

	return err
}
