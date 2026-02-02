package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/internal/config"
)

// MCPConfigStructure mimics the structure in liteclaw.extras.json
type MCPConfigStructure struct {
	MCPServers map[string]MCPServerDefinition `json:"mcpServers"`
}

type MCPServerDefinition struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// KnownRegistry holds metadata for known servers to improve usability
var KnownRegistry = map[string]struct {
	Description string
	DefaultArgs []string
	RequiredEnv []string
}{
	"@modelcontextprotocol/server-filesystem": {
		Description: "Filesystem access",
		DefaultArgs: []string{"."},
	},
	"@modelcontextprotocol/server-github": {
		Description: "GitHub API access",
		RequiredEnv: []string{"GITHUB_PERSONAL_ACCESS_TOKEN"},
	},
	"@modelcontextprotocol/server-brave-search": {
		Description: "Brave Search API",
		RequiredEnv: []string{"BRAVE_API_KEY"},
	},
	"@modelcontextprotocol/server-sqlite": {
		Description: "SQLite Database",
		DefaultArgs: []string{"test.db"},
	},
	"@modelcontextprotocol/server-gdrive": {
		Description: "Google Drive Access",
	},
	"@modelcontextprotocol/server-postgres": {
		Description: "PostgreSQL Database",
		DefaultArgs: []string{"postgresql://localhost/postgres"},
	},
}

// NewMCPCommand creates the mcp command family
func NewMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP servers",
		Long:  "Install, list, and configure MCP (Model Context Protocol) servers in your setup.",
		Example: `  liteclaw mcp install @modelcontextprotocol/server-github
  liteclaw mcp list`,
	}

	cmd.AddCommand(newMCPInstallCommand())
	cmd.AddCommand(newMCPListCommand())

	return cmd
}

func newMCPInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [package-name]",
		Short: "Install a new MCP server via NPX",
		Example: `  # Install GitHub Server
  liteclaw mcp install @modelcontextprotocol/server-github

  # Install Filesystem with explicit path
  liteclaw mcp install @modelcontextprotocol/server-filesystem --args ./

  # Install Brave Search
  liteclaw mcp install @modelcontextprotocol/server-brave-search

  # Install PostgreSQL Database
  liteclaw mcp install @modelcontextprotocol/server-postgres --args "postgresql://user:password@localhost/dbname"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pkgName := args[0]
			serverArgs, _ := cmd.Flags().GetStringArray("args")
			installMCPServer(cmd, pkgName, serverArgs)
		},
	}

	cmd.Flags().StringArray("args", []string{}, "Additional arguments to pass to the server")
	return cmd
}

func newMCPListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List installed MCP servers",
		Example: `  liteclaw mcp list`,
		Run: func(cmd *cobra.Command, args []string) {
			listMCPServers(cmd)
		},
	}
}

func getMCPConfigPath() string {
	if override := os.Getenv("LITECLAW_MCP_CONFIG_PATH"); override != "" {
		return override
	}
	return config.ExtrasPath()
}

func installMCPServer(cmd *cobra.Command, pkg string, extraArgs []string) {
	out := cmd.OutOrStdout()
	configPath := getMCPConfigPath()
	fmt.Fprintf(out, "📦 Installing '%s' into %s...\n", pkg, configPath)

	// 1. Load existing config
	var configData MCPConfigStructure
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &configData); err != nil {
			fmt.Fprintf(out, "⚠️ Warning: Failed to parse existing config: %v. Creating new.\n", err)
			configData.MCPServers = make(map[string]MCPServerDefinition)
		}
	} else {
		// Create new
		configData.MCPServers = make(map[string]MCPServerDefinition)
		// Ensure dir exists
		_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	}

	if configData.MCPServers == nil {
		configData.MCPServers = make(map[string]MCPServerDefinition)
	}

	// 2. Determine Server Name
	serverName := pkg
	parts := strings.Split(pkg, "/")
	if len(parts) > 0 {
		serverName = parts[len(parts)-1]
	}
	serverName = strings.TrimPrefix(serverName, "server-")

	// 3. Prepare Args and Env
	finalArgs := append([]string{"-y", pkg}, extraArgs...)
	finalEnv := make(map[string]string)

	// Check Registry for defaults
	if meta, ok := KnownRegistry[pkg]; ok {
		fmt.Fprintf(out, "ℹ️  Identified known server: %s\n", meta.Description)
		if len(extraArgs) == 0 && len(meta.DefaultArgs) > 0 {
			fmt.Fprintf(out, "   > Applying default args: %v\n", meta.DefaultArgs)
			finalArgs = append(finalArgs, meta.DefaultArgs...)
		}
		if len(meta.RequiredEnv) > 0 {
			fmt.Fprintf(out, "⚠️  This server typically requires these env vars:\n")
			for _, env := range meta.RequiredEnv {
				fmt.Fprintf(out, "   - %s\n", env)
				finalEnv[env] = "YOUR_" + env + "_HERE"
			}
		}
	}

	// 4. Update Config
	configData.MCPServers[serverName] = MCPServerDefinition{
		Command: "npx",
		Args:    finalArgs,
		Env:     finalEnv,
	}

	// 5. Save
	newData, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		fmt.Fprintf(out, "❌ Failed to marshal config: %v\n", err)
		return
	}

	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		fmt.Fprintf(out, "❌ Failed to write config file: %v\n", err)
		return
	}

	fmt.Fprintf(out, "✅ Successfully installed '%s' as server ID '%s'.\n", pkg, serverName)
	fmt.Fprintln(out, "🚀 Restart LiteClaw Gateway to apply changes: './liteclaw gateway start'")
}

func listMCPServers(cmd *cobra.Command) {
	out := cmd.OutOrStdout()
	configPath := getMCPConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(out, "No mcp config found at %s\n", configPath)
		return
	}

	var configData MCPConfigStructure
	if err := json.Unmarshal(data, &configData); err != nil {
		fmt.Fprintf(out, "Failed to parse config: %v\n", err)
		return
	}

	fmt.Fprintf(out, "📂 MCP Servers listed in %s:\n", configPath)
	if len(configData.MCPServers) == 0 {
		fmt.Fprintln(out, "   (No servers configured)")
		return
	}

	var sortedNames []string
	for name := range configData.MCPServers {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		def := configData.MCPServers[name]
		fmt.Fprintf(out, "- %s\n", name)
		fmt.Fprintf(out, "  Command: %s %v\n", def.Command, def.Args)
		if len(def.Env) > 0 {
			var envKeys []string
			for ek := range def.Env {
				envKeys = append(envKeys, ek)
			}
			sort.Strings(envKeys)
			fmt.Fprintf(out, "  Env: %v\n", envKeys)
		}
		fmt.Fprintln(out)
	}
}
