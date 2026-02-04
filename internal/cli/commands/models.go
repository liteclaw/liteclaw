package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewModelsCommand() *cobra.Command {
	var statusJson bool
	var statusPlain bool

	cmd := &cobra.Command{
		Use:   "models",
		Short: "Model discovery, scanning, and configuration",
		Long:  "Model discovery, scanning, and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if statusJson {
				return runModelsStatus(cmd, true, false)
			}
			if statusPlain {
				return runModelsStatus(cmd, false, true)
			}
			// Default to help if no subcommands and no explicit options
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&statusJson, "status-json", false, "Output JSON (alias for `models status --json`)")
	cmd.Flags().BoolVar(&statusPlain, "status-plain", false, "Plain output (alias for `models status --plain`)")

	cmd.AddCommand(newModelsListCommand())
	cmd.AddCommand(newModelsStatusCommand())
	cmd.AddCommand(newModelsSetCommand())
	cmd.AddCommand(newModelsAliasesCommand())
	cmd.AddCommand(newModelsFallbacksCommand())
	cmd.AddCommand(newModelsScanCommand())
	cmd.AddCommand(newModelsAuthCommand())
	cmd.AddCommand(newModelsAddCommand())
	cmd.AddCommand(newModelsConfigCommand())

	return cmd
}

func newModelsAliasesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aliases",
		Short: "Manage model aliases",
		Long: `Manage model aliases for quick access.

Aliases allow you to reference models with custom short names.
For example, you can create an alias 'fast' for 'deepseek/deepseek-chat'.`,
		Example: `  liteclaw models aliases
  liteclaw models aliases add fast deepseek/deepseek-chat
  liteclaw models aliases remove fast`,
	}

	// Default action: list aliases
	cmd.RunE = func(c *cobra.Command, args []string) error {
		return runModelsAliasesList(c, false)
	}

	// Add subcommands
	cmd.AddCommand(newModelsAliasesListCommand())
	cmd.AddCommand(newModelsAliasesAddCommand())
	cmd.AddCommand(newModelsAliasesRemoveCommand())

	return cmd
}

func newModelsAliasesListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all model aliases",
		Example: "  liteclaw models aliases list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsAliasesList(cmd, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func runModelsAliasesList(cmd *cobra.Command, jsonOutput bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Collect aliases from model configs
	aliases := make(map[string]string)
	if cfg.Agents.Defaults.Models != nil {
		for modelKey, modelEntry := range cfg.Agents.Defaults.Models {
			if modelEntry.Alias != "" {
				aliases[modelEntry.Alias] = modelKey
			}
		}
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(map[string]interface{}{"aliases": aliases}, "", "  ")
		cmd.Println(string(data))
		return nil
	}

	cmd.Printf("Aliases (%d):\n", len(aliases))
	if len(aliases) == 0 {
		cmd.Println("- none")
		cmd.Println("")
		cmd.Println("Tip: Create an alias with:")
		cmd.Println("  liteclaw models aliases add <alias> <provider/model>")
		return nil
	}

	for alias, target := range aliases {
		cmd.Printf("- %s -> %s\n", alias, target)
	}

	return nil
}

func newModelsAliasesAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <alias> <provider/model>",
		Short: "Add a model alias",
		Example: `  liteclaw models aliases add fast deepseek/deepseek-chat
  liteclaw models aliases add smart anthropic/claude-sonnet-4-20250514`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := normalizeAlias(args[0])
			modelKey := args[1]

			// Validate model key format
			if !strings.Contains(modelKey, "/") {
				return fmt.Errorf("model must be in 'provider/model' format, e.g. 'deepseek/deepseek-chat'")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check for duplicate aliases
			if cfg.Agents.Defaults.Models != nil {
				for key, entry := range cfg.Agents.Defaults.Models {
					if entry.Alias == alias && key != modelKey {
						return fmt.Errorf("alias '%s' already points to '%s'", alias, key)
					}
				}
			}

			// Initialize models map if nil
			if cfg.Agents.Defaults.Models == nil {
				cfg.Agents.Defaults.Models = make(map[string]config.AgentModelMap)
			}

			// Update or create entry for the model
			entry := cfg.Agents.Defaults.Models[modelKey]
			entry.Alias = alias
			cfg.Agents.Defaults.Models[modelKey] = entry

			// Save config
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			cmd.Printf("✓ Alias '%s' -> '%s'\n", alias, modelKey)
			return nil
		},
	}
}

func newModelsAliasesRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <alias>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a model alias",
		Example: "  liteclaw models aliases remove fast",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := normalizeAlias(args[0])

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.Agents.Defaults.Models == nil {
				return fmt.Errorf("no aliases configured")
			}

			// Find and remove the alias
			found := false
			for key, entry := range cfg.Agents.Defaults.Models {
				if entry.Alias == alias {
					entry.Alias = ""
					cfg.Agents.Defaults.Models[key] = entry
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("alias not found: %s", alias)
			}

			// Save config
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			cmd.Printf("✓ Removed alias '%s'\n", alias)
			return nil
		},
	}
}

// normalizeAlias normalizes an alias string (lowercase, trim spaces).
func normalizeAlias(alias string) string {
	return strings.TrimSpace(strings.ToLower(alias))
}

func newModelsFallbacksCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "fallbacks",
		Short:   "Manage model fallback list",
		Example: "  liteclaw models fallbacks",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Fallbacks management not implemented yet.")
		},
	}
}

func newModelsScanCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "scan",
		Short:   "Scan OpenRouter/Local models for tools + images",
		Example: "  liteclaw models scan",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Scanning for models...")
			// TODO: Implement actual scanning logic (e.g. check Ollama URL, scan OpenRouter)
			cmd.Println("Scan check complete. (Mock)")
		},
	}
}

func newModelsAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage model auth profiles",
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to a provider (set API key)",
		Example: `  liteclaw models auth login moonshot
  liteclaw models auth login --provider minimax`,
		RunE: runAuthLogin,
	}
	loginCmd.Flags().StringP("provider", "p", "", "Provider ID (e.g. minimax, moonshot)")

	cmd.AddCommand(loginCmd)
	return cmd
}

func newModelsAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add [provider/model]",
		Short: "Add a new model provider or model",
		Example: `  liteclaw models add deepseek/deepseek-chat
  liteclaw models add openai/gpt-4o-mini`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				_ = cmd.Help()
				cmd.Println("Error: provider/model argument required")
				os.Exit(1)
			}
			return nil
		},
		RunE: runModelsAdd,
	}
}

func newModelsConfigCommand() *cobra.Command {
	var baseURL string
	var apiType string

	cmd := &cobra.Command{
		Use:   "configure [provider]",
		Short: "Configure an existing provider's parameters",
		Long: `Configure an existing provider's parameters such as base URL and API type.

Supported configuration options:
  --base-url    The API endpoint URL for the provider
  --api-type    The API protocol type: 'openai' or 'anthropic'

If no flags are provided, the command runs in interactive mode.`,
		Example: `  # Set Ollama to use a remote server
  liteclaw models configure ollama --base-url http://192.168.1.154:11434/v1

  # Change DeepSeek API type
  liteclaw models configure deepseek --api-type openai

  # Set both base URL and API type at once
  liteclaw models configure my-provider --base-url https://api.example.com/v1 --api-type anthropic

  # Interactive mode (prompts for each option)
  liteclaw models configure ollama`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsConfig(cmd, args[0], baseURL, apiType)
		},
	}

	cmd.Flags().StringVar(&baseURL, "base-url", "", "Set the base URL for the provider")
	cmd.Flags().StringVar(&apiType, "api-type", "", "Set the API type (openai or anthropic)")

	return cmd
}

func runModelsConfig(cmd *cobra.Command, pName, baseURL, apiType string) error {
	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pConfig, ok := cfg.Models.Providers[pName]
	if !ok {
		return fmt.Errorf("provider '%s' not found", pName)
	}

	reader := bufio.NewReader(in)
	updated := false

	// Handle baseURL
	if baseURL != "" {
		pConfig.BaseURL = baseURL
		updated = true
		_, _ = fmt.Fprintf(out, "Base URL set to: %s\n", baseURL)
	} else {
		_, _ = fmt.Fprintf(out, "Current Base URL: %s\n", pConfig.BaseURL)
		_, _ = fmt.Fprint(out, "New Base URL (leave blank to keep): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			pConfig.BaseURL = input
			updated = true
		}
	}

	// Handle apiType
	if apiType != "" {
		if apiType == "anthropic" {
			pConfig.API = "anthropic-messages"
		} else {
			pConfig.API = "openai-completions"
		}
		updated = true
		_, _ = fmt.Fprintf(out, "API Type set to: %s\n", pConfig.API)
	} else {
		_, _ = fmt.Fprintf(out, "Current API Type: %s\n", pConfig.API)
		_, _ = fmt.Fprint(out, "New API Type (openai/anthropic, leave blank to keep): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "anthropic":
			pConfig.API = "anthropic-messages"
			updated = true
		case "openai":
			pConfig.API = "openai-completions"
			updated = true
		}
	}

	if !updated {
		_, _ = fmt.Fprintln(out, "No changes made.")
		return nil
	}

	cfg.Models.Providers[pName] = pConfig
	if err := config.Save(cfg); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "Successfully updated %s\n", pName)
	return nil
}

func runModelsAdd(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()

	fullID := args[0]
	parts := strings.Split(fullID, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format: expected provider/model (e.g. grok/grok-beta)")
	}
	pName := parts[0]
	mID := parts[1]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(in)

	// Check if provider exists
	pConfig, exists := cfg.Models.Providers[pName]
	if exists {
		_, _ = fmt.Fprintf(out, "Provider '%s' already exists.\n", pName)
		_, _ = fmt.Fprintf(out, "Add model '%s' to existing provider '%s'? (Y/n): ", mID, pName)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm == "n" || confirm == "no" {
			return fmt.Errorf("operation cancelled")
		}
	} else {
		// New Provider
		_, _ = fmt.Fprintf(out, "Creating new provider: %s\n", pName)

		_, _ = fmt.Fprint(out, "Base URL: ")
		baseURL, _ := reader.ReadString('\n')
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			return fmt.Errorf("base URL required")
		}

		_, _ = fmt.Fprint(out, "API Key: ")
		var apiKey string
		if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			byteKey, _ := term.ReadPassword(int(f.Fd()))
			_, _ = fmt.Fprintln(out)
			apiKey = strings.TrimSpace(string(byteKey))
		} else {
			// For tests/non-terminal
			input, _ := reader.ReadString('\n')
			apiKey = strings.TrimSpace(input)
		}

		if apiKey == "" {
			return fmt.Errorf("api key required")
		}

		_, _ = fmt.Fprint(out, "API Type (openai/anthropic) [default: openai]: ")
		apiTypeInput, _ := reader.ReadString('\n')
		apiTypeInput = strings.TrimSpace(strings.ToLower(apiTypeInput))

		apiType := "openai-completions"
		if apiTypeInput == "anthropic" {
			apiType = "anthropic-messages"
		} else if apiTypeInput != "" && apiTypeInput != "openai" {
			_, _ = fmt.Fprintf(out, "Warning: Unknown API type input '%s', defaulting to openai-completions.\n", apiTypeInput)
		}

		// Save apiKey to env
		envKey := strings.ToUpper(pName) + "_API_KEY"
		if cfg.Env == nil {
			cfg.Env = make(map[string]string)
		}
		cfg.Env[envKey] = apiKey

		// Initialize provider config
		pConfig = config.ModelProvider{
			BaseURL: baseURL,
			API:     apiType,
			Models:  []config.ModelEntry{},
		}
	}

	// Model Configuration
	_, _ = fmt.Fprintf(out, "\nConfiguring model '%s'...\n", mID)

	_, _ = fmt.Fprint(out, "Context Window (default 128000): ")
	ctxInput, _ := reader.ReadString('\n')
	ctxInput = strings.TrimSpace(ctxInput)
	contextWindow := 128000
	if ctxInput != "" {
		if val, err := strconv.Atoi(ctxInput); err == nil {
			contextWindow = val
		}
	}

	newModel := config.ModelEntry{
		ID:            mID,
		Name:          mID, // Default display name
		ContextWindow: contextWindow,
		MaxTokens:     8192, // Safe default
		Input:         []string{"text"},
	}

	// Check if model already exists
	modelExists := false
	for i, m := range pConfig.Models {
		if m.ID == mID {
			_, _ = fmt.Fprintf(out, "Model '%s' already exists. Overwrite? (y/N): ", mID)
			confirm, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				return nil
			}
			pConfig.Models[i] = newModel
			modelExists = true
			break
		}
	}
	if !modelExists {
		pConfig.Models = append(pConfig.Models, newModel)
	}

	// Update cfg
	if cfg.Models.Providers == nil {
		cfg.Models.Providers = make(map[string]config.ModelProvider)
	}
	cfg.Models.Providers[pName] = pConfig

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Successfully added %s/%s\n", pName, mID)
	return nil
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()

	provider, _ := cmd.Flags().GetString("provider")
	if provider == "" {
		// If argument provided, use it
		if len(args) > 0 {
			provider = args[0]
		} else {
			return fmt.Errorf("provider required (use --provider or argument)")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Validate provider/model
	if strings.Contains(provider, "/") {
		parts := strings.Split(provider, "/")
		pName := parts[0]
		mID := parts[1]

		pConfig, ok := cfg.Models.Providers[pName]
		if !ok {
			return fmt.Errorf("unknown provider '%s'", pName)
		}

		found := false
		for _, m := range pConfig.Models {
			if m.ID == mID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("model '%s' not found for provider '%s'", mID, pName)
		}

		// Use just the provider name for auth
		provider = pName
	} else {
		// Verify provider exists
		if _, ok := cfg.Models.Providers[provider]; !ok {
			return fmt.Errorf("unknown provider '%s'", provider)
		}
	}

	// Prompt for key
	_, _ = fmt.Fprintf(out, "Enter API Key for %s: ", provider)
	var key string
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		bytePassword, err := term.ReadPassword(int(f.Fd()))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		_, _ = fmt.Fprintln(out) // Print newline after hidden input
		key = string(bytePassword)
	} else {
		reader := bufio.NewReader(in)
		input, _ := reader.ReadString('\n')
		key = strings.TrimSpace(input)
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	envKey := strings.ToUpper(provider) + "_API_KEY"

	// Ensure Env map exists
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}

	cfg.Env[envKey] = key

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Successfully saved API Key for %s\n", provider)
	return nil
}

func newModelsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List configuration models",
		Example: "  liteclaw models list",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Table Output
			table := tablewriter.NewWriter(cmd.OutOrStdout())
			table.SetHeader([]string{"Provider", "Model ID", "Capabilities"})
			table.SetBorder(false)
			table.SetAutoWrapText(false)

			var rows [][]string

			// Iterate providers
			for pName, pConfig := range cfg.Models.Providers {
				for _, m := range pConfig.Models {
					caps := []string{}
					if m.Reasoning {
						caps = append(caps, "reasoning")
					}
					// Input types
					caps = append(caps, m.Input...)

					rows = append(rows, []string{
						pName,
						m.ID,
						strings.Join(caps, ", "),
					})
				}
			}

			// Sort by Provider then Model ID
			sort.Slice(rows, func(i, j int) bool {
				if rows[i][0] != rows[j][0] {
					return rows[i][0] < rows[j][0]
				}
				return rows[i][1] < rows[j][1]
			})

			table.AppendBulk(rows)
			table.Render()

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nCurrently Active Model: %s\n", cfg.Agents.Defaults.Model.Primary)

			return nil
		},
	}
}

func newModelsStatusCommand() *cobra.Command {
	var jsonOutput bool
	var plainOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show configured model state",
		Example: `  liteclaw models status --json
  liteclaw models status --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsStatus(cmd, jsonOutput, plainOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	cmd.Flags().BoolVar(&plainOutput, "plain", false, "Plain output")
	return cmd
}

func runModelsStatus(cmd *cobra.Command, jsonOutput, plainOutput bool) error {
	out := cmd.OutOrStdout()
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	status := map[string]interface{}{
		"defaults":  cfg.Agents.Defaults.Model,
		"providers": getProviderStatus(cfg),
	}

	if jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	if plainOutput {
		_, _ = fmt.Fprintf(out, "Default Model: %s\n", cfg.Agents.Defaults.Model.Primary)
		return nil
	}

	// Rich Output
	_, _ = fmt.Fprintf(out, "\n🤖 Configured Models Status\n")
	_, _ = fmt.Fprintf(out, "========================\n\n")
	_, _ = fmt.Fprintf(out, "Default Model: %s\n", cfg.Agents.Defaults.Model.Primary)
	_, _ = fmt.Fprintf(out, "\nProviders:\n")

	for p, info := range status["providers"].(map[string]interface{}) {
		_, _ = fmt.Fprintf(out, "- %s: %v\n", p, info)
	}
	_, _ = fmt.Fprintln(out)

	return nil
}

func getProviderStatus(cfg *config.Config) map[string]interface{} {
	// Check auth/env for each provider
	res := make(map[string]interface{})

	// Normalize config env keys to upper for lookup
	normalizedEnv := make(map[string]string)
	for k, v := range cfg.Env {
		normalizedEnv[strings.ToUpper(k)] = v
	}

	for pName := range cfg.Models.Providers {
		targetKey := strings.ToUpper(pName) + "_API_KEY"
		hasKey := false

		// 1. Check OS Env (Case-sensitive usually, but usually Upper)
		if os.Getenv(targetKey) != "" {
			hasKey = true
		} else {
			// 2. Check Config Env (Normalized)
			if val, ok := normalizedEnv[targetKey]; ok && val != "" {
				hasKey = true
			}
		}

		res[pName] = map[string]bool{
			"configured": true,
			"has_key":    hasKey,
		}
	}
	return res
}

func newModelsSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "set [model]",
		Short:   "Set the default model",
		Example: "  liteclaw models set minimax/MiniMax-M2.1",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			modelRef := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Validation Logic
			parts := strings.Split(modelRef, "/")
			if len(parts) != 2 {
				// TODO: Check Aliases here if implemented
				return fmt.Errorf("invalid format '%s'. usage: <provider>/<model_id> (e.g. moonshot/kimi-k2-0905-preview)", modelRef)
			}

			providerName := parts[0]
			modelID := parts[1]

			pConfig, ok := cfg.Models.Providers[providerName]
			if !ok {
				return fmt.Errorf("unknown provider '%s'. Run 'liteclaw models list' to see available providers", providerName)
			}

			found := false
			for _, m := range pConfig.Models {
				if m.ID == modelID {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("model '%s' not found for provider '%s'. Run 'liteclaw models list' to see available models", modelID, providerName)
			}

			_, _ = fmt.Fprintf(out, "Setting default model to: %s\n", modelRef)
			cfg.Agents.Defaults.Model.Primary = modelRef
			return config.Save(cfg)
		},
	}
}
