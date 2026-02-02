// Package commands provides CLI subcommands for LiteClaw.
package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/gateway"
)

// NewGatewayCommand creates the gateway subcommand.
func NewGatewayCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the LiteClaw gateway server",
		Long:  `Start, stop, and manage the LiteClaw gateway server.`,
		Example: `  liteclaw gateway start -d
  liteclaw gateway status`,
	}

	// Add persistent flags that work for parent and subcommands
	cmd.PersistentFlags().IntP("port", "p", 18789, "Gateway port")
	cmd.PersistentFlags().String("host", "0.0.0.0", "Gateway host")
	cmd.PersistentFlags().BoolP("detached", "d", false, "Run in background")

	// Subcommands
	cmd.AddCommand(newGatewayStartCommand())
	cmd.AddCommand(newGatewayStopCommand())
	cmd.AddCommand(newGatewayStatusCommand())
	cmd.AddCommand(newGatewayRestartCommand())

	// Default action: start the gateway
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGatewayStart(cmd, args)
	}

	return cmd
}

func newGatewayStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the gateway server",
		Example: `  # Foreground default
  liteclaw gateway start

  # Background with custom host/port
  liteclaw gateway start --detached --host 0.0.0.0 --port 9090`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGatewayStart(cmd, args)
		},
	}

	return cmd
}

func newGatewayStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "stop",
		Short:   "Stop the gateway server",
		Example: `  liteclaw gateway stop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGatewayStop(cmd)
		},
	}
}

func newGatewayStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show gateway server status",
		Example: `  liteclaw gateway status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGatewayStatus(cmd)
		},
	}
}

func newGatewayRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "restart",
		Short:   "Restart the gateway server",
		Example: `  liteclaw gateway restart`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGatewayRestart(cmd)
		},
	}
}

func runGatewayStart(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			fmt.Fprintln(out, "❌ No LiteClaw config found.")
			fmt.Fprintln(out, "   Run: liteclaw onboard")
			return err
		}
		fmt.Fprintf(out, "Warning: Failed to load config: %v\n", err)
		cfg = &config.Config{}
	}

	// 2. Parse Flags
	portFlag, _ := cmd.Flags().GetInt("port")
	hostFlag, _ := cmd.Flags().GetString("host")
	detached, _ := cmd.Flags().GetBool("detached")

	port := 18789 // Default
	if cmd.Flags().Changed("port") {
		port = portFlag
	} else if cfg.Gateway.Port > 0 {
		port = cfg.Gateway.Port
	}

	host := "0.0.0.0"
	if cmd.Flags().Changed("host") {
		host = hostFlag
	} else {
		if cfg.Gateway.Bind == "loopback" {
			host = "127.0.0.1"
		} else if cfg.Gateway.Bind == "public" || cfg.Gateway.Bind == "0.0.0.0" {
			host = "0.0.0.0"
		}
	}

	// 3. Handle Detached Mode
	if detached {
		if err := ensureGatewayNotRunning(); err != nil {
			return err
		}

		logDir := filepath.Join(config.StateDir(), "logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log dir: %w", err)
		}
		logPath := filepath.Join(logDir, "gateway.log")

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		executable, err := os.Executable()
		if err != nil {
			executable = "liteclaw" // Fallback
		}

		// Construct args: "gateway", "start", explicit flags
		// Do NOT pass --detached to avoid infinite loop
		childArgs := []string{"gateway", "start", "--port", fmt.Sprintf("%d", port), "--host", host}

		c := exec.Command(executable, childArgs...)
		c.Stdout = logFile
		c.Stderr = logFile

		if err := c.Start(); err != nil {
			logFile.Close()
			return fmt.Errorf("failed to start background process: %w", err)
		}

		// Close file in parent
		logFile.Close()

		fmt.Fprintf(out, "培育... LiteClaw Gateway started in background (PID: %d)\n", c.Process.Pid)
		fmt.Fprintf(out, "Logs: %s\n", logPath)
		fmt.Fprintln(out, "Use 'liteclaw logs' to view logs.")
		return nil // Success, naturally exit
	}

	// 4. Single Instance Check (Foreground process)
	lockDir := config.ConfigDir()
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}
	lockPath := filepath.Join(lockDir, "liteclaw-gateway.lock")
	fileLock := flock.New(lockPath)

	locked, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("error checking lock file: %w", err)
	}

	if !locked {
		fmt.Fprintln(out, "❌ Error: LiteClaw Gateway is already running.")
		fmt.Fprintf(out, "   Lock file found at: %s\n", lockPath)
		fmt.Fprintln(out, "   Only one instance of the gateway service is allowed to prevent port conflicts")
		fmt.Fprintln(out, "   and duplicate connections to chat platforms.")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "   To check running processes:")
		fmt.Fprintln(out, "     ps aux | grep liteclaw")
		fmt.Fprintln(out, "")
		return fmt.Errorf("gateway already running")
	}
	defer func() { _ = fileLock.Unlock() }()

	if err := writeGatewayPID(); err != nil {
		return err
	}
	defer func() { _ = removeGatewayPID() }()

	fmt.Fprintf(out, "🦞 Starting LiteClaw Gateway on %s:%d\n", host, port)

	server := gateway.New(&gateway.Config{
		Host: host,
		Port: port,
	})

	// For tests, skip actual start if configured
	if os.Getenv("LITECLAW_SKIP_GATEWAY_START") == "true" {
		fmt.Fprintln(out, "Skipping actual server start for testing.")
		return nil
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}
	return nil
}

func runGatewayStop(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	pid, err := readGatewayPID()
	if err != nil {
		return fmt.Errorf("gateway not running (pid file missing)")
	}

	if !checkProcessRunning(pid) {
		_ = removeGatewayPID()
		return fmt.Errorf("gateway process not running (stale pid file)")
	}

	if err := terminateProcess(pid); err != nil {
		return fmt.Errorf("failed to stop gateway (pid %d): %w", pid, err)
	}

	fmt.Fprintf(out, "Sent stop signal to gateway (PID %d)\n", pid)
	waitForProcessExit(pid, 3*time.Second)
	return nil
}

func runGatewayStatus(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	port := cfg.Gateway.Port
	if port == 0 {
		port = 18789
	}

	status, err := fetchGatewayStatus("127.0.0.1", port)
	if err != nil {
		fmt.Fprintln(out, "Gateway: not running")
		return nil
	}

	fmt.Fprintf(out, "Gateway: %s (uptime %s)\n", status.Status, status.Uptime)
	return nil
}

func runGatewayRestart(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Restarting gateway server...")
	if err := runGatewayStop(cmd); err != nil {
		fmt.Fprintf(out, "Warning: stop failed (%v), continuing to start...\n", err)
	}

	return runGatewayStart(cmd, nil)
}

func gatewayPIDPath() string {
	return filepath.Join(config.StateDir(), "liteclaw-gateway.pid")
}

func writeGatewayPID() error {
	if err := os.MkdirAll(config.StateDir(), 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}
	pidPath := gatewayPIDPath()
	pid := strconv.Itoa(os.Getpid())
	return os.WriteFile(pidPath, []byte(pid), 0644)
}

func readGatewayPID() (int, error) {
	data, err := os.ReadFile(gatewayPIDPath())
	if err != nil {
		return 0, err
	}
	pidStr := string(data)
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		return 0, fmt.Errorf("invalid pid file")
	}
	return pid, nil
}

func removeGatewayPID() error {
	return os.Remove(gatewayPIDPath())
}

func ensureGatewayNotRunning() error {
	lockDir := config.ConfigDir()
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}
	lockPath := filepath.Join(lockDir, "liteclaw-gateway.lock")
	fileLock := flock.New(lockPath)
	locked, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("error checking lock file: %w", err)
	}
	if !locked {
		return fmt.Errorf("gateway already running")
	}
	_ = fileLock.Unlock()
	return nil
}

// waitForExit moved to process_unix.go and process_windows.go as waitForProcessExit
