package cron

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "liteclaw-cron")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storePath := filepath.Join(tmpDir, "jobs.json")
	logger := zerolog.Nop()

	// 1. Create and Add
	s1 := NewScheduler(storePath, logger)
	job := &Job{
		ID:      "job-1",
		Name:    "Test Job",
		Enabled: true,
		Schedule: Schedule{
			Kind:    ScheduleKindEvery,
			EveryMs: 60000,
		},
		Payload: Payload{
			Kind: PayloadKindSystemEvent,
			Text: "ping",
		},
	}
	err = s1.AddJob(job)
	require.NoError(t, err)

	// 2. Load in new instance
	s2 := NewScheduler(storePath, logger)
	err = s2.Load()
	require.NoError(t, err)

	jobs := s2.Jobs()
	require.Len(t, jobs, 1)
	assert.Equal(t, "job-1", jobs[0].ID)
	assert.Equal(t, "Test Job", jobs[0].Name)
	assert.Equal(t, int64(60000), jobs[0].Schedule.EveryMs)
}

func TestScheduler_Lifecycle(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler("", logger) // Memory only

	job := &Job{
		ID:      "job-1",
		Enabled: true,
		Schedule: Schedule{
			Kind: ScheduleKindCron,
			Expr: "* * * * * *",
		},
	}

	// Add
	err := s.AddJob(job)
	require.NoError(t, err)
	retrieved, ok := s.GetJob("job-1")
	require.True(t, ok)
	assert.NotZero(t, retrieved.cronEntryID)

	// Update (Disable)
	job.Enabled = false
	err = s.UpdateJob(job)
	require.NoError(t, err)
	retrieved, _ = s.GetJob("job-1")
	assert.Zero(t, retrieved.cronEntryID)

	// Update (Enable)
	job.Enabled = true
	err = s.UpdateJob(job)
	require.NoError(t, err)
	retrieved, _ = s.GetJob("job-1")
	assert.NotZero(t, retrieved.cronEntryID)

	// Remove
	err = s.RemoveJob("job-1")
	require.NoError(t, err)
	_, ok = s.GetJob("job-1")
	require.False(t, ok)
}

func TestScheduler_Execution(t *testing.T) {
	logger := zerolog.Nop()
	s := NewScheduler("", logger)

	executed := make(chan bool, 1)
	s.SetExecutor(func(ctx context.Context, job *Job) error {
		executed <- true
		return nil
	})
	s.Start()
	defer s.Stop()

	// Add immediate job logic is hard to test deterministically via Schedule
	// But we can use RunJobNow
	job := &Job{
		ID:      "job-exec",
		Enabled: true,
		Schedule: Schedule{
			Kind:    ScheduleKindEvery,
			EveryMs: 1000000,
		},
	}
	_ = s.AddJob(job)

	_ = s.RunJobNow("job-exec")

	select {
	case <-executed:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Job execution timed out")
	}

	// Allow time for async state update to complete before reading
	time.Sleep(100 * time.Millisecond)

	// Verify state updated
	j, _ := s.GetJob("job-exec")
	assert.Equal(t, "ok", j.State.LastStatus)
}
