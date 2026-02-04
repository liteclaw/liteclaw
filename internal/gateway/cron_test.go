package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/liteclaw/liteclaw/internal/agent"
	"github.com/liteclaw/liteclaw/internal/cron"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCronHandlers(t *testing.T) {
	// Setup Scheduler
	tmpDir, err := os.MkdirTemp("", "gateway-cron-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	logger := zerolog.Nop()
	sched := cron.NewScheduler(tmpDir+"/jobs.json", logger)

	// Setup Server
	server := New(&Config{Host: "localhost", Port: 0})

	// Inject Scheduler
	server.agentService = &agent.Service{
		Scheduler: sched,
	}

	e := echo.New()
	e.Validator = NewCustomValidator()

	// Add Job via API
	t.Run("AddJob", func(t *testing.T) {
		job := cron.Job{
			Name:     "API Job",
			Enabled:  true,
			Schedule: cron.Schedule{Kind: cron.ScheduleKindEvery, EveryMs: 10000},
			Payload:  cron.Payload{Kind: cron.PayloadKindSystemEvent, Text: "ping"},
		}
		body, _ := json.Marshal(job)
		req := httptest.NewRequest(http.MethodPost, "/api/cron/jobs", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := server.handleCronAdd(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		j := resp["job"].(map[string]interface{})
		assert.Equal(t, "API Job", j["name"])
		assert.NotEmpty(t, j["id"])
	})

	// List Jobs
	t.Run("ListJobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/cron/jobs", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := server.handleCronList(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		jobs := resp["jobs"].([]interface{})
		assert.Len(t, jobs, 1)
	})

	// Run Job
	t.Run("RunJob", func(t *testing.T) {
		// First get the ID
		reqL := httptest.NewRequest(http.MethodGet, "/api/cron/jobs", nil)
		recL := httptest.NewRecorder()
		cL := e.NewContext(reqL, recL)
		_ = server.handleCronList(cL)
		var respL map[string]interface{}
		_ = json.Unmarshal(recL.Body.Bytes(), &respL)
		jobs := respL["jobs"].([]interface{})
		id := jobs[0].(map[string]interface{})["id"].(string)

		req := httptest.NewRequest(http.MethodPost, "/api/cron/jobs/"+id+"/run", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(id)

		err := server.handleCronRun(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
