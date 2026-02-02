// Package cron provides scheduled task functionality.
package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

// ScheduleKind defines the type of schedule.
type ScheduleKind string

const (
	ScheduleKindAt    ScheduleKind = "at"
	ScheduleKindEvery ScheduleKind = "every"
	ScheduleKindCron  ScheduleKind = "cron"
)

// Schedule defines when a job should run.
type Schedule struct {
	Kind     ScheduleKind `json:"kind" validate:"required,oneof=at every cron"`
	AtMs     int64        `json:"atMs,omitempty"`     // for "at"
	EveryMs  int64        `json:"everyMs,omitempty"`  // for "every"
	AnchorMs int64        `json:"anchorMs,omitempty"` // for "every" alignment
	Expr     string       `json:"expr,omitempty"`     // for "cron"
}

// PayloadKind defines what the job does when fired.
type PayloadKind string

const (
	PayloadKindSystemEvent PayloadKind = "systemEvent"
	PayloadKindAgentTurn   PayloadKind = "agentTurn"
)

// Payload defines the job execution details.
type Payload struct {
	Kind                       PayloadKind `json:"kind" validate:"required,oneof=systemEvent agentTurn"`
	Text                       string      `json:"text,omitempty"`    // for "systemEvent"
	Message                    string      `json:"message,omitempty"` // for "agentTurn"
	Model                      string      `json:"model,omitempty"`   // for "agentTurn"
	Channel                    string      `json:"channel,omitempty"` // for "agentTurn" (target channel)
	To                         string      `json:"to,omitempty"`      // for "agentTurn" (target recipient)
	AllowUnsafeExternalContent bool        `json:"allowUnsafeExternalContent,omitempty"`
	Deliver                    bool        `json:"deliver,omitempty"`        // If true, deliver response to user
	TimeoutSeconds             int         `json:"timeoutSeconds,omitempty"` // Execution timeout
}

// JobState tracks the runtime state of a job.
type JobState struct {
	NextRunAtMs    int64  `json:"nextRunAtMs,omitempty"`
	RunningAtMs    int64  `json:"runningAtMs,omitempty"`
	LastRunAtMs    int64  `json:"lastRunAtMs,omitempty"`
	LastStatus     string `json:"lastStatus,omitempty"` // "ok", "error", "skipped"
	LastError      string `json:"lastError,omitempty"`
	LastDurationMs int64  `json:"lastDurationMs,omitempty"`
}

// Job represents a scheduled job.
type Job struct {
	ID             string   `json:"id"`
	AgentID        string   `json:"agentId,omitempty"` // e.g. "main"
	Name           string   `json:"name,omitempty"`
	Description    string   `json:"description,omitempty"`
	Enabled        bool     `json:"enabled"`
	DeleteAfterRun bool     `json:"deleteAfterRun,omitempty"`
	CreatedAtMs    int64    `json:"createdAtMs"`
	UpdatedAtMs    int64    `json:"updatedAtMs"`
	Schedule       Schedule `json:"schedule"`
	Payload        Payload  `json:"payload"`
	SessionTarget  string   `json:"sessionTarget"`      // "main" | "isolated"
	WakeMode       string   `json:"wakeMode,omitempty"` // "next-heartbeat" | "now"
	State          JobState `json:"state"`

	// Runtime only - can be cancelled
	cronEntryID cron.EntryID `json:"-"`
}

// StoreFile represents the JSON structure for persistence.
type StoreFile struct {
	Version int    `json:"version"`
	Jobs    []*Job `json:"jobs"`
}

// JobExecutor is the delegate function that executes a job.
type JobExecutor func(ctx context.Context, job *Job) error

// Scheduler manages scheduled jobs.
type Scheduler struct {
	cron      *cron.Cron
	jobs      map[string]*Job
	storePath string
	executor  JobExecutor
	logger    zerolog.Logger

	mu      sync.RWMutex
	running bool
}

// NewScheduler creates a new scheduler.
func NewScheduler(storePath string, logger zerolog.Logger) *Scheduler {
	// Ensure directory exists
	if storePath != "" {
		_ = os.MkdirAll(filepath.Dir(storePath), 0755)
	}

	s := &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		jobs:      make(map[string]*Job),
		storePath: storePath,
		logger:    logger.With().Str("component", "cron").Logger(),
	}
	return s
}

// SetExecutor sets the function that will execute triggered jobs.
func (s *Scheduler) SetExecutor(exec JobExecutor) {
	s.executor = exec
}

// Load reads jobs from disk.
func (s *Scheduler) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var store StoreFile
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}

	for _, job := range store.Jobs {
		s.jobs[job.ID] = job
		// Schedule it if enabled
		if job.Enabled {
			_ = s.scheduleJob(job)
		}
	}
	s.logger.Info().Int("count", len(s.jobs)).Msg("Loaded cron jobs")
	return nil
}

// Save writes jobs to disk.
func (s *Scheduler) Save() error {
	if s.storePath == "" {
		return nil
	}

	var jobsList []*Job
	for _, j := range s.jobs {
		jobsList = append(jobsList, j)
	}
	store := StoreFile{
		Version: 1,
		Jobs:    jobsList,
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.storePath, data, 0644)
}

// AddJob adds a new job and saves it.
func (s *Scheduler) AddJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job %s already exists", job.ID)
	}

	if job.CreatedAtMs == 0 {
		job.CreatedAtMs = time.Now().UnixMilli()
	}
	job.UpdatedAtMs = time.Now().UnixMilli()

	s.jobs[job.ID] = job
	if job.Enabled {
		if err := s.scheduleJob(job); err != nil {
			return err
		}
	}

	return s.saveLocked()
}

// RemoveJob removes a job and saves.
func (s *Scheduler) RemoveJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil
	}

	if job.cronEntryID != 0 {
		s.cron.Remove(job.cronEntryID)
	}

	delete(s.jobs, id)
	return s.saveLocked()
}

// internal helper to save while lock is held
func (s *Scheduler) saveLocked() error {
	// Temporarily unlock to call Save (which locks), or refactor Save.
	// Actually Save locks, so we should duplicate logic or make Save take no lock?
	// Let's just reimplement saving logic here to avoid recursive lock issues,
	// or specific saveInternal that assumes lock held.

	if s.storePath == "" {
		return nil
	}

	var jobsList []*Job
	for _, j := range s.jobs {
		jobsList = append(jobsList, j)
	}
	store := StoreFile{
		Version: 1,
		Jobs:    jobsList,
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.storePath, data, 0644)
}

// scheduleJob registers the job with the underlying cron engine.
// Expects lock to be held.
func (s *Scheduler) scheduleJob(job *Job) error {
	// Clear existing entry if any
	if job.cronEntryID != 0 {
		s.cron.Remove(job.cronEntryID)
		job.cronEntryID = 0
	}

	var err error
	var entryID cron.EntryID

	wrappedRun := func() {
		s.execJobWrapper(job.ID)
	}

	switch job.Schedule.Kind {
	case ScheduleKindCron:
		if job.Schedule.Expr == "" {
			return fmt.Errorf("empty cron expression")
		}
		entryID, err = s.cron.AddFunc(job.Schedule.Expr, wrappedRun)

	case ScheduleKindEvery:
		if job.Schedule.EveryMs <= 0 {
			return fmt.Errorf("invalid everyMs")
		}
		// Go cron doesn't strictly support "every X ms" with millisecond precision nicely via standard string
		// But robfig/cron supports "@every 1h30m10s". We can convert ms to duration string.
		dur := time.Duration(job.Schedule.EveryMs) * time.Millisecond
		spec := fmt.Sprintf("@every %s", dur.String())
		entryID, err = s.cron.AddFunc(spec, wrappedRun)

	case ScheduleKindAt:
		// Logic handles "At" by calculating delay
		now := time.Now().UnixMilli()
		delay := job.Schedule.AtMs - now
		if delay <= 0 {
			// run immediately
			go wrappedRun()
			return nil // No recurring entry
		}
		time.AfterFunc(time.Duration(delay)*time.Millisecond, wrappedRun)
		return nil
	default:
		return fmt.Errorf("unknown schedule kind")
	}

	if err != nil {
		return err
	}
	job.cronEntryID = entryID

	// Update NextRunAtMs
	entry := s.cron.Entry(entryID)
	if !entry.Next.IsZero() {
		job.State.NextRunAtMs = entry.Next.UnixMilli()
	}

	return nil
}

// execJobWrapper calls the executor and updates state.
func (s *Scheduler) execJobWrapper(jobID string) {
	s.mu.RLock()
	job, exists := s.jobs[jobID]
	s.mu.RUnlock()

	if !exists || !job.Enabled {
		return
	}

	// Update state: Running (with lock)
	start := time.Now()
	s.mu.Lock()
	job.State.RunningAtMs = start.UnixMilli()
	s.mu.Unlock()

	s.logger.Info().Str("job", job.Name).Msg("Executing scheduled job")

	var err error
	if s.executor != nil {
		// Run with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err = s.executor(ctx, job)
	} else {
		err = fmt.Errorf("no executor configured")
	}

	// Update state: Finished (with lock)
	duration := time.Since(start).Milliseconds()
	s.mu.Lock()
	job.State.LastRunAtMs = start.UnixMilli()
	job.State.RunningAtMs = 0
	job.State.LastDurationMs = duration
	if err != nil {
		job.State.LastStatus = "error"
		job.State.LastError = err.Error()
		s.logger.Error().Err(err).Str("job", job.Name).Msg("Job execution failed")
	} else {
		job.State.LastStatus = "ok"
		job.State.LastError = ""
		s.logger.Info().Str("job", job.Name).Msg("Job execution completed")
	}

	// Update NextRunAtMs for the next cycle
	if job.cronEntryID != 0 {
		entry := s.cron.Entry(job.cronEntryID)
		if !entry.Next.IsZero() {
			job.State.NextRunAtMs = entry.Next.UnixMilli()
		} else {
			job.State.NextRunAtMs = 0
		}
	}

	// Save state (already holding lock)
	_ = s.saveLocked()
	s.mu.Unlock()

	// Handle DeleteAfterRun for "at" tasks or one-offs
	if job.DeleteAfterRun || job.Schedule.Kind == ScheduleKindAt {
		_ = s.RemoveJob(job.ID)
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.cron.Start()
	s.running = true
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.cron.Stop()
	s.running = false
}

// Jobs returns all registered jobs.
func (s *Scheduler) Jobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		list = append(list, j)
	}
	return list
}

// IsRunning returns whether the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// RunJobNow triggers a job immediately.
func (s *Scheduler) RunJobNow(id string) error {
	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	// Execute asynchronously
	go s.execJobWrapper(job.ID)
	return nil
}

// UpdateJob updates an existing job.
func (s *Scheduler) UpdateJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	old, exists := s.jobs[job.ID]
	if !exists {
		return fmt.Errorf("job %s does not exist", job.ID)
	}

	job.CreatedAtMs = old.CreatedAtMs
	job.UpdatedAtMs = time.Now().UnixMilli()

	// Handle reschedule
	if job.Enabled {
		// scheduleJob handles clearing old entry
		if err := s.scheduleJob(job); err != nil {
			return err
		}
	} else {
		// Stop if running
		if old.cronEntryID != 0 {
			s.cron.Remove(old.cronEntryID)
		}
		job.cronEntryID = 0
	}

	s.jobs[job.ID] = job
	return s.saveLocked()
}

// GetJob returns a job by ID.
func (s *Scheduler) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}
