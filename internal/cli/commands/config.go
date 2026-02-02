package commands

import (
	"strconv"
	"strings"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/spf13/cobra"
)

func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Config helpers (get/set/unset)",
		Long:  `Get and set configuration values in the active config file.`,
		Example: `  # Get config value
  liteclaw config get gateway.port

  # Set config value
  liteclaw config set gateway.port 8080`,
	}

	cmd.AddCommand(newConfigGetCommand())
	cmd.AddCommand(newConfigSetCommand())

	return cmd
}

func newConfigGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "get [key]",
		Short:   "Get a configuration value",
		Example: `  liteclaw config get gateway.port`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize config to load file
			v, err := config.LoadViper()
			if err != nil {
				cmd.Printf("Failed to load config: %v\n", err)
				return
			}

			key := args[0]
			val := v.Get(key)
			if val == nil {
				cmd.Println("null")
				return
			}
			cmd.Printf("%v\n", val)
		},
	}
}

func newConfigSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Example: `  liteclaw config set gateway.port 9000
  liteclaw config set logging.printSystemPrompt true`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize config
			v, err := config.LoadViper()
			if err != nil {
				cmd.Printf("Failed to load config: %v\n", err)
				return
			}

			key := args[0]
			valStr := args[1]
			var val interface{} = valStr

			// Type inference attempt
			if vInt, err := strconv.Atoi(valStr); err == nil {
				val = vInt
			} else if vBool, err := strconv.ParseBool(valStr); err == nil {
				val = vBool
			} else if strings.HasPrefix(valStr, "[") || strings.HasPrefix(valStr, "{") {
				// Detected JSON array/object, keep as string for now
				val = valStr
			}

			v.Set(key, val)

			if err := v.WriteConfig(); err != nil {
				// If WriteConfig fails, try WriteConfigAs with the determined path
				target := v.ConfigFileUsed()
				if target == "" {
					target = config.ConfigPath()
				}
				if err := v.WriteConfigAs(target); err != nil {
					cmd.Printf("Failed to write config: %v\n", err)
					return
				}
			}

			cmd.Printf("Updated %s = %v\n", key, val)
		},
	}
}
