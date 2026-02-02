package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liteclaw/liteclaw/internal/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCronListCommand(t *testing.T) {
	// Mock gateway
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/cron/jobs" {
			jobs := []*cron.Job{
				{
					ID:      "job-1",
					Name:    "Periodic Ping",
					Enabled: true,
					Schedule: cron.Schedule{
						Kind:    cron.ScheduleKindEvery,
						EveryMs: 60000,
					},
					State: cron.JobState{
						LastStatus: "success",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobs})
		}
	}))
	defer server.Close()

	url := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(url, ":")

	// Mock config to use this gateway - isolate from real ~/.liteclaw
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")
	configContent := fmt.Sprintf(`{"gateway":{"port":%s},"agents":{"defaults":{"workspace":"%s"}}}`, parts[1], tempDir)
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Set both config path AND state dir to isolate from real config
	os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer os.Unsetenv("LITECLAW_CONFIG_PATH")
	defer os.Unsetenv("LITECLAW_STATE_DIR")

	cmd := newCronListCommand()

	out := CaptureStdout(func() {
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, out, "job-1")
	assert.Contains(t, out, "Periodic Ping")
	assert.Contains(t, out, "every 1m")
	assert.Contains(t, out, "success")
}

func TestCronAddCommand(t *testing.T) {
	var capturedJob cron.Job
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/cron/jobs" {
			_ = json.NewDecoder(r.Body).Decode(&capturedJob)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok"}`))
		}
	}))
	defer server.Close()

	url := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(url, ":")

	// Isolate from real ~/.liteclaw
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "liteclaw.json")
	configContent := fmt.Sprintf(`{"gateway":{"port":%s},"agents":{"defaults":{"workspace":"%s"}}}`, parts[1], tempDir)
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	os.Setenv("LITECLAW_CONFIG_PATH", configPath)
	os.Setenv("LITECLAW_STATE_DIR", tempDir)
	defer os.Unsetenv("LITECLAW_CONFIG_PATH")
	defer os.Unsetenv("LITECLAW_STATE_DIR")

	cmd := newCronAddCommand()

	out := CaptureStdout(func() {
		cmd.SetArgs([]string{
			"--name", "New Job",
			"--every", "5m",
			"--message", "HelloWorld",
		})
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, out, "Job added successfully")
	assert.Equal(t, "New Job", capturedJob.Name)
	assert.Equal(t, int64(300000), capturedJob.Schedule.EveryMs)
	assert.Equal(t, "HelloWorld", capturedJob.Payload.Message)
}
