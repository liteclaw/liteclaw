// Package commands provides CLI subcommands for LiteClaw.
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/gateway"
)

const (
	defaultGatewayHost  = "127.0.0.1"
	fallbackGatewayPort = 18789
	statusTimeout       = 2 * time.Second
)

// GatewayStatusResponse matches the StatusResponse from handlers.go
type GatewayStatusResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	Sessions  int    `json:"sessions"`
	GoVersion string `json:"goVersion"`
	Arch      string `json:"arch"`
	OS        string `json:"os"`
	Channels  []struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		Connected bool   `json:"connected"`
		Sessions  int    `json:"sessions"`
	} `json:"channels"`
	Memory struct {
		Alloc      uint64 `json:"alloc"`
		TotalAlloc uint64 `json:"totalAlloc"`
		Sys        uint64 `json:"sys"`
		NumGC      uint32 `json:"numGC"`
	} `json:"memory"`
}

// NewStatusCommand creates the status subcommand.
func NewStatusCommand() *cobra.Command {
	var (
		host       string
		port       int
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show LiteClaw status",
		Long:  `Display the current status of LiteClaw including gateway state, channels, and sessions.`,
		Example: `  liteclaw status
  liteclaw status --host 127.0.0.1 --port 18789 --json`,
		Run: func(cmd *cobra.Command, args []string) {
			// If port not explicitly set, try to load from config
			actualPort := port
			if actualPort == 0 {
				if cfg, err := config.Load(); err == nil && cfg.Gateway.Port > 0 {
					actualPort = cfg.Gateway.Port
				} else {
					actualPort = fallbackGatewayPort
				}
			}
			runStatus(cmd.OutOrStdout(), host, actualPort, jsonOutput)
		},
	}

	cmd.Flags().StringVar(&host, "host", defaultGatewayHost, "Gateway host")
	cmd.Flags().IntVar(&port, "port", 0, "Gateway port (default: from config file, or 18789)")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")

	return cmd
}

func runStatus(out io.Writer, host string, port int, jsonOutput bool) {
	// Try to connect to the gateway
	status, err := fetchGatewayStatus(host, port)

	if jsonOutput {
		if err != nil {
			fmt.Fprintf(out, `{"running": false, "error": "%s"}`, err.Error())
			fmt.Fprintln(out)
			return
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Fprintln(out, string(data))
		return
	}

	fmt.Fprintln(out, "🦞 LiteClaw Status")
	fmt.Fprintln(out, "================")
	fmt.Fprintln(out)

	if err != nil {
		fmt.Fprintln(out, "Gateway:   ✗ Not running")
		fmt.Fprintln(out, "Channels:  -")
		fmt.Fprintln(out, "Sessions:  -")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Start the gateway with: liteclaw gateway start")
		return
	}

	fmt.Fprintf(out, "Gateway:   ✓ Running on %s:%d\n", host, port)
	fmt.Fprintf(out, "Version:   %s\n", status.Version)
	fmt.Fprintf(out, "Uptime:    %s\n", status.Uptime)
	fmt.Fprintf(out, "Sessions:  %d active\n", status.Sessions)

	// Show connected channels
	if len(status.Channels) > 0 {
		connected := 0
		var channelNames []string
		for _, ch := range status.Channels {
			if ch.Connected {
				connected++
				channelNames = append(channelNames, ch.Name)
			}
		}
		if connected > 0 {
			fmt.Fprintf(out, "Channels:  %d connected (%s)\n", connected, formatList(channelNames))
		} else {
			fmt.Fprintf(out, "Channels:  %d registered, 0 connected\n", len(status.Channels))
		}
	} else {
		fmt.Fprintln(out, "Channels:  0 configured")
	}

	// Show memory stats
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Memory:    %s alloc, %s sys\n",
		formatBytes(status.Memory.Alloc),
		formatBytes(status.Memory.Sys))
	fmt.Fprintf(out, "Runtime:   %s (%s/%s)\n", status.GoVersion, status.OS, status.Arch)
	fmt.Fprintln(out)
}

func fetchGatewayStatus(host string, port int) (*GatewayStatusResponse, error) {
	client := &http.Client{Timeout: statusTimeout}

	// Try /api/status first
	url := fmt.Sprintf("http://%s:%d/api/status", host, port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Load token and add to request
	token, _ := gateway.LoadClawdbotToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned status %d", resp.StatusCode)
	}

	var status GatewayStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

func formatList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	if len(items) <= 3 {
		result := items[0]
		for i := 1; i < len(items); i++ {
			result += ", " + items[i]
		}
		return result
	}
	return fmt.Sprintf("%s, %s, +%d more", items[0], items[1], len(items)-2)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
