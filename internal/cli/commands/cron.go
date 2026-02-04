package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/liteclaw/liteclaw/internal/agent"
	"github.com/liteclaw/liteclaw/internal/agent/workspace"
	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/cron"
	"github.com/liteclaw/liteclaw/internal/gateway"
	"github.com/spf13/cobra"
)

// NewCronCommand creates the cron command
func NewCronCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Manage cron jobs (via Gateway)",
		Long:  `Manage scheduled tasks. Requires Gateway to be running.`,
		Example: `  liteclaw cron list
  liteclaw cron add --name "Test" --every 1h --message "Ping"`,
	}

	cmd.AddCommand(newCronListCommand())
	cmd.AddCommand(newCronAddCommand())
	cmd.AddCommand(newCronRmCommand())
	cmd.AddCommand(newCronUpdateCommand())
	cmd.AddCommand(newCronEnableCommand())
	cmd.AddCommand(newCronDisableCommand())
	cmd.AddCommand(newCronRunCommand())
	cmd.AddCommand(newCronHistoryCommand())

	return cmd
}

// -----------------------------------------------------------------------------
// List
// -----------------------------------------------------------------------------

func newCronListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List cron jobs",
		Example: `  liteclaw cron list`,
		Run: func(cmd *cobra.Command, args []string) {
			_, jobs, err := fetchCronList(cmd.OutOrStdout())
			if err != nil {
				cmd.Printf("Error listing jobs: %v\n", err)
				return
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tNAME\tENABLED\tSCHEDULE\tLAST RUN\tSTATUS")
			for _, j := range jobs {
				sched := "?"
				switch j.Schedule.Kind {
				case cron.ScheduleKindCron:
					sched = j.Schedule.Expr
				case cron.ScheduleKindEvery:
					sched = fmt.Sprintf("every %dms", j.Schedule.EveryMs)
					if dur := time.Duration(j.Schedule.EveryMs) * time.Millisecond; dur > time.Second {
						sched = fmt.Sprintf("every %s", dur.String())
					}
				case cron.ScheduleKindAt:
					sched = fmt.Sprintf("at %s", time.UnixMilli(j.Schedule.AtMs).Format(time.RFC3339))
				}

				lastRun := "-"
				if j.State.LastRunAtMs > 0 {
					when := time.UnixMilli(j.State.LastRunAtMs)
					lastRun = when.Format("01-02 15:04:05")
				}

				status := j.State.LastStatus
				if status == "" {
					status = "pending"
				}
				if j.State.RunningAtMs > 0 {
					status = "RUNNING"
				}

				enabled := "No"
				if j.Enabled {
					enabled = "Yes"
				}

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", j.ID, j.Name, enabled, sched, lastRun, status)
			}
			_ = w.Flush()
		},
	}
	return cmd
}

// -----------------------------------------------------------------------------
// Add
// -----------------------------------------------------------------------------

func newCronAddCommand() *cobra.Command {
	var (
		name, desc  string
		disabled    bool
		deleteAfter bool
		at, every   string
		cronExpr    string
		sysEvent    string
		message     string
		channel, to string
		deliver     bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a cron job",
		Example: `  # Schedule a message every morning at 9am to Telegram
  liteclaw cron add --name "Morning Greeting" --cron "0 9 * * *" --message "Good morning! summarize news." --channel telegram --to 123456789 --deliver

  # Schedule a system maintenance check every 30 minutes (disabled initially)
  liteclaw cron add --name "System Check" --every 30m --system-event "perform_check" --disabled --description "Regular maintenance"

  # Run a one-time task in 10 minutes and delete after execution
  liteclaw cron add --name "OneShot" --at 10m --message "Remind me" --delete-after-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validation
			schedCount := 0
			if at != "" {
				schedCount++
			}
			if every != "" {
				schedCount++
			}
			if cronExpr != "" {
				schedCount++
			}

			if schedCount != 1 {
				return fmt.Errorf("must specify exactly one of --at, --every, --cron")
			}

			if sysEvent == "" && message == "" {
				return fmt.Errorf("must specify --system-event or --message")
			}

			// Construct Job
			job := cron.Job{
				Name:           name,
				Description:    desc,
				Enabled:        !disabled,
				DeleteAfterRun: deleteAfter,
				Payload: cron.Payload{
					Deliver: deliver,
					Channel: channel,
					To:      to,
				},
			}

			// Schedule
			if at != "" {
				job.Schedule.Kind = cron.ScheduleKindAt
				// Try parsing strict format (ISO) or Duration
				if dur, err := time.ParseDuration(at); err == nil {
					job.Schedule.AtMs = time.Now().Add(dur).UnixMilli()
				} else if t, err := time.Parse(time.RFC3339, at); err == nil {
					job.Schedule.AtMs = t.UnixMilli()
				} else {
					return fmt.Errorf("invalid --at format (use ISO8601 or duration like 10m)")
				}
			} else if every != "" {
				job.Schedule.Kind = cron.ScheduleKindEvery
				dur, err := time.ParseDuration(every)
				if err != nil {
					return fmt.Errorf("invalid --every duration: %v", err)
				}
				job.Schedule.EveryMs = dur.Milliseconds()
			} else {
				job.Schedule.Kind = cron.ScheduleKindCron
				job.Schedule.Expr = cronExpr
			}

			// Payload
			if sysEvent != "" {
				job.Payload.Kind = cron.PayloadKindSystemEvent
				job.Payload.Text = sysEvent
			} else {
				job.Payload.Kind = cron.PayloadKindAgentTurn
				job.Payload.Message = message
			}

			// Send to API
			if err := addCronJob(cmd.OutOrStdout(), &job); err != nil {
				return err
			}
			fmt.Println("Job added successfully.")
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&name, "name", "", "Job name (required)")
	f.StringVar(&desc, "description", "", "Job description")
	f.BoolVar(&disabled, "disabled", false, "Start disabled")
	f.BoolVar(&deleteAfter, "delete-after-run", false, "Delete after successful run")

	f.StringVar(&at, "at", "", "Run once at time (ISO) or duration")
	f.StringVar(&every, "every", "", "Run every duration")
	f.StringVar(&cronExpr, "cron", "", "Cron expression")

	f.StringVar(&sysEvent, "system-event", "", "System event payload")
	f.StringVarP(&message, "message", "m", "", "Agent message payload")

	f.StringVar(&channel, "channel", "last", "Delivery channel")
	f.StringVar(&to, "to", "", "Delivery recipient")
	f.BoolVar(&deliver, "deliver", false, "Deliver response")

	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// -----------------------------------------------------------------------------
// Remove
// -----------------------------------------------------------------------------

func newCronRmCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "rm [id]",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove a cron job",
		Example: `  liteclaw cron rm 550e8400-e29b-41d4-a716-446655440000`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := removeCronJob(cmd.OutOrStdout(), id); err != nil {
				return err
			}
			return nil
		},
	}
}

// -----------------------------------------------------------------------------
// Update
// -----------------------------------------------------------------------------

func newCronUpdateCommand() *cobra.Command {
	var (
		name, desc  string
		enabled     *bool
		at, every   string
		cronExpr    string
		sysEvent    string
		message     string
		channel, to string
		deliver     bool
	)

	cmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update a cron job",
		Example: `  liteclaw cron update <id> --name "New Name"
  liteclaw cron update <id> --every 10m --message "Ping" --channel telegram`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			patch := map[string]interface{}{}

			if name != "" {
				patch["name"] = name
			}
			if desc != "" {
				patch["description"] = desc
			}
			if enabled != nil {
				patch["enabled"] = *enabled
			}

			schedCount := 0
			if at != "" {
				schedCount++
			}
			if every != "" {
				schedCount++
			}
			if cronExpr != "" {
				schedCount++
			}
			if schedCount > 1 {
				return fmt.Errorf("must specify only one of --at, --every, --cron")
			}

			if schedCount == 1 {
				var schedule cron.Schedule
				if at != "" {
					schedule.Kind = cron.ScheduleKindAt
					if dur, err := time.ParseDuration(at); err == nil {
						schedule.AtMs = time.Now().Add(dur).UnixMilli()
					} else if t, err := time.Parse(time.RFC3339, at); err == nil {
						schedule.AtMs = t.UnixMilli()
					} else {
						return fmt.Errorf("invalid --at format (use ISO8601 or duration like 10m)")
					}
				} else if every != "" {
					schedule.Kind = cron.ScheduleKindEvery
					dur, err := time.ParseDuration(every)
					if err != nil {
						return fmt.Errorf("invalid --every duration: %v", err)
					}
					schedule.EveryMs = dur.Milliseconds()
				} else {
					schedule.Kind = cron.ScheduleKindCron
					schedule.Expr = cronExpr
				}
				patch["schedule"] = schedule
			}

			payloadChange := sysEvent != "" || message != ""
			if payloadChange {
				payload := cron.Payload{
					Channel: channel,
					To:      to,
					Deliver: deliver,
				}
				if sysEvent != "" {
					payload.Kind = cron.PayloadKindSystemEvent
					payload.Text = sysEvent
				} else {
					payload.Kind = cron.PayloadKindAgentTurn
					payload.Message = message
				}
				patch["payload"] = payload
			} else if channel != "" || to != "" || deliver {
				return fmt.Errorf("payload update requires --message or --system-event")
			}

			if len(patch) == 0 {
				return fmt.Errorf("no fields to update")
			}

			if err := updateJobWithPatch(cmd.OutOrStdout(), id, patch); err != nil {
				return err
			}
			fmt.Println("Job updated successfully.")
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&name, "name", "", "Job name")
	f.StringVar(&desc, "description", "", "Job description")
	f.BoolVar(&deliver, "deliver", false, "Deliver response")

	f.StringVar(&at, "at", "", "Run once at time (ISO) or duration")
	f.StringVar(&every, "every", "", "Run every duration")
	f.StringVar(&cronExpr, "cron", "", "Cron expression")

	f.StringVar(&sysEvent, "system-event", "", "System event payload")
	f.StringVarP(&message, "message", "m", "", "Agent message payload")

	f.StringVar(&channel, "channel", "", "Delivery channel")
	f.StringVar(&to, "to", "", "Delivery recipient")

	f.Bool("enable", false, "Enable job")
	f.Bool("disable", false, "Disable job")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		en, _ := cmd.Flags().GetBool("enable")
		dis, _ := cmd.Flags().GetBool("disable")
		if en && dis {
			return fmt.Errorf("--enable and --disable are mutually exclusive")
		}
		if en {
			v := true
			enabled = &v
		}
		if dis {
			v := false
			enabled = &v
		}
		return nil
	}

	return cmd
}

// -----------------------------------------------------------------------------
// Enable/Disable (Update)
// -----------------------------------------------------------------------------

func newCronEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "enable [id]",
		Short:   "Enable a cron job",
		Example: `  liteclaw cron enable <id>`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return updateJob(id, map[string]interface{}{"enabled": true})
		},
	}
}

func newCronDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "disable [id]",
		Short:   "Disable a cron job",
		Example: `  liteclaw cron disable <id>`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return updateJob(id, map[string]interface{}{"enabled": false})
		},
	}
}

func updateJob(id string, patch map[string]interface{}) error {
	return updateJobWithPatch(os.Stdout, id, patch)
}

func updateJobWithPatch(out io.Writer, id string, patch map[string]interface{}) error {
	path := fmt.Sprintf("/cron/jobs/%s/update", id)
	if err := callCronAPI("POST", path, patch, nil); err == nil {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}
	storePath := resolveCronStorePath(cfg)
	if err := updateCronJobLocal(storePath, id, patch); err != nil {
		return fmt.Errorf("failed to update local cron file: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Warning: gateway not reachable; updated local cron file: %s\n", storePath)
	return nil
}

// -----------------------------------------------------------------------------
// Run
// -----------------------------------------------------------------------------

func newCronRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "run [id]",
		Short:   "Trigger a cron job immediately",
		Example: `  liteclaw cron run <id>`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return runCronJob(cmd.OutOrStdout(), id)
		},
	}
}

// -----------------------------------------------------------------------------
// History
// -----------------------------------------------------------------------------

func newCronHistoryCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "runs [id]", // TS alias 'runs'
		Short:   "Show run history (last state)",
		Example: `  liteclaw cron runs <id>`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return showCronHistory(cmd.OutOrStdout(), id)
		},
	}
}

// -----------------------------------------------------------------------------
// Helper
// -----------------------------------------------------------------------------

func fetchCronList(out io.Writer) (*config.Config, []cron.Job, error) {
	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}

	port := cfg.Gateway.Port
	if port == 0 {
		port = 18789
	}
	url := fmt.Sprintf("http://%s:%d/api/cron/jobs", "127.0.0.1", port)
	req, err := http.NewRequest("GET", url, nil)
	if err == nil {
		// Add token authentication
		token, _ := gateway.LoadClawdbotToken()
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode == 200 {
				var payload struct {
					Jobs []cron.Job `json:"jobs"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
					return cfg, payload.Jobs, nil
				}
			} else {
				b, _ := io.ReadAll(resp.Body)
				_, _ = fmt.Fprintf(out, "Warning: gateway error (%d): %s\n", resp.StatusCode, string(b))
			}
		} else {
			_, _ = fmt.Fprintf(out, "Warning: failed to contact gateway: %v\n", err)
		}
	}

	// Fallback: read local cron store file
	storePath := resolveCronStorePath(cfg)
	jobs, err := loadCronJobsFromFile(storePath)
	if err != nil {
		return cfg, nil, fmt.Errorf("failed to read local cron file: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Note: gateway not reachable; loaded local cron file: %s\n", storePath)
	return cfg, jobs, nil
}

func resolveCronStorePath(cfg *config.Config) string {
	workspaceDir := cfg.Agents.Defaults.Workspace
	if strings.TrimSpace(workspaceDir) == "" {
		workspaceDir = workspace.ResolveDefaultDir()
	}
	return filepath.Join(workspaceDir, "data", "cron_jobs.json")
}

func loadCronJobsFromFile(path string) ([]cron.Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []cron.Job{}, nil
		}
		return nil, err
	}
	var store cron.StoreFile
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	var jobs []cron.Job
	for _, j := range store.Jobs {
		if j != nil {
			jobs = append(jobs, *j)
		}
	}
	return jobs, nil
}

func addCronJob(out io.Writer, job *cron.Job) error {
	if err := callCronAPI("POST", "/cron/jobs", job, nil); err == nil {
		return nil
	} else {
		_, _ = fmt.Fprintf(out, "Warning: failed to contact gateway, falling back to local file: %v\n", err)
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}
	storePath := resolveCronStorePath(cfg)
	if err := addCronJobToFile(storePath, job); err != nil {
		return fmt.Errorf("failed to add job to local file: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Added to local cron file: %s\n", storePath)
	return nil
}

func addCronJobToFile(path string, job *cron.Job) error {
	store, err := loadCronStoreFile(path)
	if err != nil {
		return err
	}
	for _, existing := range store.Jobs {
		if existing != nil && existing.ID == job.ID && job.ID != "" {
			return fmt.Errorf("job %s already exists", job.ID)
		}
	}
	now := time.Now().UnixMilli()
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.CreatedAtMs == 0 {
		job.CreatedAtMs = now
	}
	job.UpdatedAtMs = now
	store.Jobs = append(store.Jobs, job)
	return writeCronStoreFile(path, store)
}

func loadCronStoreFile(path string) (*cron.StoreFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cron.StoreFile{Version: 1, Jobs: []*cron.Job{}}, nil
		}
		return nil, err
	}
	var store cron.StoreFile
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	if store.Version == 0 {
		store.Version = 1
	}
	return &store, nil
}

func writeCronStoreFile(path string, store *cron.StoreFile) error {
	updated, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, updated, 0644)
}

func removeCronJob(out io.Writer, id string) error {
	path := fmt.Sprintf("/cron/jobs/%s", id)
	if err := callCronAPI("DELETE", path, nil, nil); err == nil {
		return nil
	} else {
		_, _ = fmt.Fprintf(out, "Warning: failed to contact gateway, falling back to local file: %v\n", err)
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}
	storePath := resolveCronStorePath(cfg)
	if err := removeCronJobFromFile(storePath, id); err != nil {
		return fmt.Errorf("failed to remove job from local file: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Removed from local cron file: %s\n", storePath)
	return nil
}

func removeCronJobFromFile(path, id string) error {
	store, err := loadCronStoreFile(path)
	if err != nil {
		return err
	}

	changed := false
	filtered := make([]*cron.Job, 0, len(store.Jobs))
	for _, job := range store.Jobs {
		if job == nil || job.ID == id {
			if job != nil {
				changed = true
			}
			continue
		}
		filtered = append(filtered, job)
	}
	if !changed {
		return fmt.Errorf("job not found in local file: %s", id)
	}
	store.Jobs = filtered
	return writeCronStoreFile(path, store)
}

func updateCronJobLocal(path, id string, patch map[string]interface{}) error {
	store, err := loadCronStoreFile(path)
	if err != nil {
		return err
	}
	found := false
	for _, job := range store.Jobs {
		if job == nil || job.ID != id {
			continue
		}
		if err := applyCronPatchToJob(job, patch); err != nil {
			return err
		}
		job.UpdatedAtMs = time.Now().UnixMilli()
		found = true
		break
	}
	if !found {
		return fmt.Errorf("job not found in local file: %s", id)
	}
	return writeCronStoreFile(path, store)
}

func applyCronPatchToJob(job *cron.Job, patch map[string]interface{}) error {
	if v, ok := patch["enabled"]; ok {
		if b, ok := v.(bool); ok {
			job.Enabled = b
		}
	}
	if v, ok := patch["name"].(string); ok && v != "" {
		job.Name = v
	}
	if v, ok := patch["description"].(string); ok {
		job.Description = v
	}
	if v, ok := patch["schedule"]; ok {
		switch val := v.(type) {
		case cron.Schedule:
			job.Schedule = val
		default:
			b, err := json.Marshal(val)
			if err != nil {
				return err
			}
			var sched cron.Schedule
			if err := json.Unmarshal(b, &sched); err != nil {
				return err
			}
			job.Schedule = sched
		}
	}
	if v, ok := patch["payload"]; ok {
		switch val := v.(type) {
		case cron.Payload:
			job.Payload = val
		default:
			b, err := json.Marshal(val)
			if err != nil {
				return err
			}
			var payload cron.Payload
			if err := json.Unmarshal(b, &payload); err != nil {
				return err
			}
			job.Payload = payload
		}
	}
	return nil
}

func runCronJob(out io.Writer, id string) error {
	path := fmt.Sprintf("/cron/jobs/%s/run", id)
	if err := callCronAPI("POST", path, nil, nil); err == nil {
		return nil
	} else {
		_, _ = fmt.Fprintf(out, "Warning: failed to contact gateway, trying local execution: %v\n", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config for local run: %w", err)
	}
	storePath := resolveCronStorePath(cfg)
	return runCronJobLocal(out, storePath, cfg, id)
}

func runCronJobLocal(out io.Writer, path string, cfg *config.Config, id string) error {
	store, err := loadCronStoreFile(path)
	if err != nil {
		return err
	}
	var job *cron.Job
	for _, j := range store.Jobs {
		if j != nil && j.ID == id {
			job = j
			break
		}
	}
	if job == nil {
		return fmt.Errorf("job not found in local file: %s", id)
	}
	if job.Payload.Kind != cron.PayloadKindAgentTurn {
		return fmt.Errorf("local run only supports agentTurn jobs")
	}
	if strings.TrimSpace(job.Payload.Message) == "" {
		return fmt.Errorf("job has empty message")
	}

	start := time.Now()
	svc := agent.NewService(cfg, nil)
	var resp strings.Builder
	err = svc.ProcessChat(context.Background(), "cron:"+job.ID, job.Payload.Message, func(delta string) {
		resp.WriteString(delta)
	})
	job.State.LastRunAtMs = time.Now().UnixMilli()
	job.State.LastDurationMs = time.Since(start).Milliseconds()
	if err != nil {
		job.State.LastStatus = "error"
		job.State.LastError = err.Error()
	} else {
		job.State.LastStatus = "ok"
		job.State.LastError = ""
	}
	job.UpdatedAtMs = time.Now().UnixMilli()

	if err := writeCronStoreFile(path, store); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Local run complete (status: %s). Response length: %d\n", job.State.LastStatus, resp.Len())
	return err
}

func showCronHistory(out io.Writer, id string) error {
	path := fmt.Sprintf("/cron/jobs/%s/history", id)
	var resp struct {
		History []cron.JobState `json:"history"`
	}
	if err := callCronAPI("GET", path, nil, &resp); err == nil {
		return printCronHistory(out, resp.History)
	} else {
		_, _ = fmt.Fprintf(out, "Warning: failed to contact gateway, falling back to local file: %v\n", err)
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}
	storePath := resolveCronStorePath(cfg)
	store, err := loadCronStoreFile(storePath)
	if err != nil {
		return fmt.Errorf("failed to read local cron file: %w", err)
	}
	for _, job := range store.Jobs {
		if job != nil && job.ID == id {
			return printCronHistory(out, []cron.JobState{job.State})
		}
	}
	return fmt.Errorf("job not found in local file: %s", id)
}

func printCronHistory(out io.Writer, history []cron.JobState) error {
	if len(history) == 0 {
		_, _ = fmt.Fprintln(out, "No history available.")
		return nil
	}
	s := history[0]
	if s.LastRunAtMs == 0 {
		_, _ = fmt.Fprintln(out, "No history available.")
		return nil
	}
	_, _ = fmt.Fprintf(out, "Last Run: %s\n", time.UnixMilli(s.LastRunAtMs).Format(time.RFC3339))
	_, _ = fmt.Fprintf(out, "Status:   %s\n", s.LastStatus)
	_, _ = fmt.Fprintf(out, "Duration: %dms\n", s.LastDurationMs)
	if s.LastError != "" {
		_, _ = fmt.Fprintf(out, "Error:    %s\n", s.LastError)
	}
	return nil
}

func callCronAPI(method, path string, body interface{}, result interface{}) error {
	cfg, _ := config.Load() // Ignore error, use defaults
	port := 18789
	if cfg != nil && cfg.Gateway.Port > 0 {
		port = cfg.Gateway.Port
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/api%s", port, path)

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return err
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add token authentication
	token, _ := gateway.LoadClawdbotToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(b))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
