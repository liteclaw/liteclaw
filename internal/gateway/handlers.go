// Package gateway provides the LiteClaw gateway server.
package gateway

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/liteclaw/liteclaw/internal/cron"
	"github.com/liteclaw/liteclaw/internal/version"
)

// StatusResponse represents the gateway status.
type StatusResponse struct {
	Status    string          `json:"status"`
	Version   string          `json:"version"`
	Uptime    string          `json:"uptime"`
	Memory    MemoryStats     `json:"memory"`
	Channels  []ChannelStatus `json:"channels"`
	Sessions  int             `json:"sessions"`
	GoVersion string          `json:"goVersion"`
	Arch      string          `json:"arch"`
	OS        string          `json:"os"`
}

// MemoryStats represents memory usage.
type MemoryStats struct {
	Alloc      uint64 `json:"alloc"`      // Bytes allocated and in use
	TotalAlloc uint64 `json:"totalAlloc"` // Total bytes allocated
	Sys        uint64 `json:"sys"`        // Bytes obtained from system
	NumGC      uint32 `json:"numGC"`      // Number of GC cycles
}

// ChannelStatus represents a channel's status.
type ChannelStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
	Sessions  int    `json:"sessions"`
}

// handleHealth handles GET /health
func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// handleRoot handles GET /
func (s *Server) handleRoot(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"name":    "LiteClaw Gateway",
		"version": version.Version,
		"status":  "running",
	})
}

// handleStatus handles GET /api/status
func (s *Server) handleStatus(c echo.Context) error {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Build channel status from adapters
	channels := make([]ChannelStatus, 0, len(s.adapters))
	for name, adapter := range s.adapters {
		channels = append(channels, ChannelStatus{
			Name:      name,
			Type:      name,
			Connected: adapter.IsConnected(),
			Sessions:  0, // TODO: Get from session manager
		})
	}

	resp := StatusResponse{
		Status:  "running",
		Version: version.Version,
		Uptime:  s.Uptime().Round(time.Second).String(),
		Memory: MemoryStats{
			Alloc:      memStats.Alloc,
			TotalAlloc: memStats.TotalAlloc,
			Sys:        memStats.Sys,
			NumGC:      memStats.NumGC,
		},
		Channels:  channels,
		Sessions:  s.sessionManager.SessionCount(),
		GoVersion: runtime.Version(),
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
	}

	return c.JSON(http.StatusOK, resp)
}

// handleListSessions handles GET /api/sessions
func (s *Server) handleListSessions(c echo.Context) error {
	// TODO: Implement session listing
	return c.JSON(http.StatusOK, map[string]interface{}{
		"sessions": []interface{}{},
		"total":    0,
	})
}

// handleSendToSession handles POST /api/sessions/:id/send
func (s *Server) handleSendToSession(c echo.Context) error {
	sessionID := c.Param("id")
	// TODO: Implement sending message to session
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":        true,
		"sessionId": sessionID,
	})
}

// handleListChannels handles GET /api/channels
func (s *Server) handleListChannels(c echo.Context) error {
	// TODO: Implement channel listing
	return c.JSON(http.StatusOK, map[string]interface{}{
		"channels": []interface{}{},
	})
}

// handleGetConfig handles GET /api/config
func (s *Server) handleGetConfig(c echo.Context) error {
	// TODO: Implement config retrieval
	return c.JSON(http.StatusOK, map[string]interface{}{
		"config": map[string]interface{}{},
	})
}

// handleUpdateConfig handles POST /api/config
func (s *Server) handleUpdateConfig(c echo.Context) error {
	// TODO: Implement config update
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "Config updated",
	})
}

// handleRestart handles POST /api/gateway/restart
func (s *Server) handleRestart(c echo.Context) error {
	s.logger.Info().Msg("Restart requested via API")
	// TODO: Implement restart logic
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "Restarting...",
	})
}

// handleReload handles POST /api/gateway/reload
func (s *Server) handleReload(c echo.Context) error {
	s.logger.Info().Msg("Reload requested via API")
	// TODO: Implement reload logic
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "Configuration reloaded",
	})
}

// Cron Handlers

func (s *Server) handleCronList(c echo.Context) error {
	jobs := s.agentService.GetScheduler().Jobs()
	return c.JSON(http.StatusOK, map[string]interface{}{"jobs": jobs})
}

func (s *Server) handleCronGet(c echo.Context) error {
	id := c.Param("id")
	job, ok := s.agentService.GetScheduler().GetJob(id)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"job": job})
}

func (s *Server) handleCronAdd(c echo.Context) error {
	var job cron.Job
	if err := c.Bind(&job); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	// Validate request body
	if err := c.Validate(&job); err != nil {
		return err // Echo handles HTTPError from CustomValidator
	}
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if err := s.agentService.GetScheduler().AddJob(&job); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"job": job})
}

func (s *Server) handleCronRemove(c echo.Context) error {
	id := c.Param("id")
	if err := s.agentService.GetScheduler().RemoveJob(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCronUpdate(c echo.Context) error {
	id := c.Param("id")
	scheduler := s.agentService.GetScheduler()

	job, ok := scheduler.GetJob(id)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}

	// Simple partial update via map
	var patch map[string]interface{}
	if err := json.NewDecoder(c.Request().Body).Decode(&patch); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
	}

	// TS CLI wraps in "patch"
	if p, ok := patch["patch"]; ok {
		if pm, ok := p.(map[string]interface{}); ok {
			patch = pm
		}
	}

	if v, ok := patch["enabled"]; ok {
		if b, ok := v.(bool); ok {
			job.Enabled = b
		}
	}
	// Allow payload/schedule updates if provided
	// (Requires marshalling back to struct to be safe, but minimal for now)

	if err := scheduler.UpdateJob(job); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"job": job})
}

func (s *Server) handleCronRun(c echo.Context) error {
	id := c.Param("id")
	if err := s.agentService.GetScheduler().RunJobNow(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCronHistory(c echo.Context) error {
	id := c.Param("id")
	job, ok := s.agentService.GetScheduler().GetJob(id)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "job not found"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"history": []cron.JobState{job.State},
	})
}
