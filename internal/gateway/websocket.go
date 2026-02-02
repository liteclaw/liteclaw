package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/cron"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handleWebSocket(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.logger.Error().Err(err).Msg("WebSocket upgrade failed")
		return err
	}
	defer ws.Close()

	s.logger.Info().Msg("WebSocket client connected")

	for {
		// Read message
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.logger.Info().Msg("WebSocket client disconnected")
			} else {
				s.logger.Error().Err(err).Msg("WebSocket read error")
			}
			break
		}

		s.logger.Info().Str("msg", string(msg)).Msg("Recv")

		// Parse Request
		var req struct {
			Type   string                 `json:"type"`
			ID     string                 `json:"id"`
			Method string                 `json:"method"`
			Params map[string]interface{} `json:"params"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			s.logger.Error().Err(err).Msg("Failed to unmarshal request")
			continue
		}

		// Handle specific methods
		switch req.Method {
		case "connect":
			// Handshake response with Uptime
			uptimeMs := s.Uptime().Milliseconds()
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"type":     "hello-ok",
					"protocol": 1,
					"server": map[string]string{
						"version": "dev",
					},
					"snapshot": map[string]interface{}{
						"uptimeMs": uptimeMs,
					},
				},
			}
			_ = ws.WriteJSON(res)

		case "config.get":
			// Use ConfigPath() for consistency
			path := config.ConfigPath()
			content, err := os.ReadFile(path)
			exists := true
			if err != nil {
				exists = false
				content = []byte("{}")
			}

			// Hash for optimistic locking
			h := sha256.New()
			h.Write(content)
			hash := hex.EncodeToString(h.Sum(nil))

			// Parse for snapshot
			var cfgObj interface{}
			_ = json.Unmarshal(content, &cfgObj)

			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"exists": exists,
					"valid":  true,
					"path":   path,
					"raw":    string(content),
					"hash":   hash,
					"config": cfgObj,
				},
			}
			_ = ws.WriteJSON(res)

		case "config.set":
			raw, _ := req.Params["raw"].(string)

			// Validation
			var cfg config.Config
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				_ = ws.WriteJSON(map[string]interface{}{
					"type":  "res",
					"id":    req.ID,
					"ok":    false,
					"error": map[string]interface{}{"message": "Invalid JSON format: " + err.Error()},
				})
				break
			}

			if err := cfg.Validate(); err != nil {
				_ = ws.WriteJSON(map[string]interface{}{
					"type":  "res",
					"id":    req.ID,
					"ok":    false,
					"error": map[string]interface{}{"message": "Config validation failed: " + err.Error()},
				})
				break
			}

			// Use ConfigPath() for consistency
			path := config.ConfigPath()
			err := os.WriteFile(path, []byte(raw), 0600)
			if err != nil {
				_ = ws.WriteJSON(map[string]interface{}{
					"type":  "res",
					"id":    req.ID,
					"ok":    false,
					"error": map[string]interface{}{"message": err.Error()},
				})
				break
			}

			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"ok":   true,
					"path": path,
				},
			}
			_ = ws.WriteJSON(res)

		case "config.schema":
			// Stub: return minimal schema
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			}
			_ = ws.WriteJSON(res)

		case "cron.status":
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"enabled": true,
					"running": true,
				},
			}
			_ = ws.WriteJSON(res)

		case "cron.list":
			jobs := s.agentService.GetScheduler().Jobs()
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"jobs": jobs,
				},
			}
			_ = ws.WriteJSON(res)

		case "cron.add":
			jobArg, ok := req.Params["job"].(map[string]interface{})
			if !ok {
				_ = ws.WriteJSON(map[string]interface{}{"type": "res", "id": req.ID, "ok": false, "error": "job param required"})
				break
			}
			jobJSON, _ := json.Marshal(jobArg)
			var job cron.Job
			if err := json.Unmarshal(jobJSON, &job); err != nil {
				_ = ws.WriteJSON(map[string]interface{}{"type": "res", "id": req.ID, "ok": false, "error": "invalid job json: " + err.Error()})
				break
			}

			// Defaults
			if job.ID == "" {
				job.ID = uuid.New().String()
			}
			if _, ok := jobArg["enabled"]; !ok {
				job.Enabled = true
			}
			if job.AgentID == "" {
				job.AgentID = "main"
			}
			if job.WakeMode == "" {
				job.WakeMode = "next-heartbeat"
			}
			if job.SessionTarget == "" {
				job.SessionTarget = "isolated"
			}
			if job.Payload.TimeoutSeconds == 0 {
				job.Payload.TimeoutSeconds = 10 // Default to 10s (matches TS)
			}
			// Default Model if missing
			if job.Payload.Model == "" {
				job.Payload.Model = s.agentService.Config.Agents.Defaults.Model.Primary
			}

			// Default Deliver to true for agentTurn
			if job.Payload.Kind == cron.PayloadKindAgentTurn {
				// We can't easily distinguish between "false" and "unset" for bool without pointer
				// But generally creating a new job implies delivery.
				// TS implementation defaults bestEffortDeliver=true.
				// Let's force it true if not explicitly set in the *incoming map* (which we have in jobArg)
				payloadMap, _ := jobArg["payload"].(map[string]interface{})
				if _, set := payloadMap["deliver"]; !set {
					job.Payload.Deliver = true
				}
			}

			if err := s.agentService.GetScheduler().AddJob(&job); err != nil {
				_ = ws.WriteJSON(map[string]interface{}{"type": "res", "id": req.ID, "ok": false, "error": "failed to add job: " + err.Error()})
				break
			}

			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"ok":  true,
					"job": job,
				},
			}
			_ = ws.WriteJSON(res)

		case "cron.remove":
			id, _ := req.Params["id"].(string)
			if id == "" {
				_ = ws.WriteJSON(map[string]interface{}{"type": "res", "id": req.ID, "ok": false, "error": "id param required"})
				break
			}
			if err := s.agentService.GetScheduler().RemoveJob(id); err != nil {
				_ = ws.WriteJSON(map[string]interface{}{"type": "res", "id": req.ID, "ok": false, "error": "failed to remove job: " + err.Error()})
				break
			}
			_ = ws.WriteJSON(map[string]interface{}{
				"type":    "res",
				"id":      req.ID,
				"ok":      true,
				"payload": map[string]interface{}{"ok": true, "id": id},
			})

		case "cron.run":
			id, _ := req.Params["id"].(string)
			if id == "" {
				_ = ws.WriteJSON(map[string]interface{}{"type": "res", "id": req.ID, "ok": false, "error": "id param required"})
				break
			}
			if err := s.agentService.GetScheduler().RunJobNow(id); err != nil {
				_ = ws.WriteJSON(map[string]interface{}{"type": "res", "id": req.ID, "ok": false, "error": "failed to run job: " + err.Error()})
				break
			}
			_ = ws.WriteJSON(map[string]interface{}{
				"type":    "res",
				"id":      req.ID,
				"ok":      true,
				"payload": map[string]interface{}{"ok": true, "id": id},
			})

		case "node.list":
			hostname, _ := os.Hostname()
			nodes := []map[string]interface{}{
				{
					"nodeId":      "liteclaw-local",
					"displayName": hostname,
					"connected":   true,
					"paired":      true,
					"platform":    runtime.GOOS,
				},
			}
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"ts":    time.Now().UnixMilli(),
					"nodes": nodes,
				},
			}
			_ = ws.WriteJSON(res)

		case "channels.list":
			// Stub: Return empty channels list
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"channels":     []interface{}{},
					"channelOrder": []string{},
				},
			}
			_ = ws.WriteJSON(res)

		case "agents.list":
			cfg := s.agentService.Config
			agentsList := make([]map[string]interface{}, 0)
			// Return the primary agent from config as long as we don't have a multi-agent list yet
			// In liteclaw.json, agents.list might exist.
			// For now, let's return a basic list with what we have.
			agentsList = append(agentsList, map[string]interface{}{
				"id":    "main",
				"name":  "Clawdbot",
				"model": cfg.Agents.Defaults.Model.Primary,
			})

			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"agents": agentsList,
				},
			}
			_ = ws.WriteJSON(res)

		case "presence.list":
			// Stub: Return empty presence list
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"entries": []interface{}{},
				},
			}
			_ = ws.WriteJSON(res)

		case "sessions.list":
			sessions := s.sessionManager.ListSessions()
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"ts":       time.Now().UnixMilli(),
					"path":     s.sessionManager.baseDir,
					"sessions": sessions,
					"count":    len(sessions),
					"defaults": map[string]interface{}{
						"thinkingLevel": "off",
					},
				},
			}
			_ = ws.WriteJSON(res)

		case "chat.history":
			sessionKey, _ := req.Params["sessionKey"].(string)

			msgs, err := s.sessionManager.GetHistory(sessionKey)
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to get chat history")
				msgs = []Message{}
			}

			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"sessionKey":    sessionKey,
					"messages":      msgs,
					"thinkingLevel": "off",
				},
			}
			_ = ws.WriteJSON(res)

		case "chat.send":
			// extract params
			message, _ := req.Params["message"].(string)
			sessionKey, _ := req.Params["sessionKey"].(string)
			runId, _ := req.Params["idempotencyKey"].(string)
			if runId == "" {
				runId = req.ID // fallback
			}

			// 0. Log incoming TUI message
			s.logger.Info().
				Str("session", sessionKey).
				Str("text", message).
				Msg("TUI message received")

			// Store User Message in Persistent History
			_ = s.sessionManager.AddMessage(sessionKey, "user", message)

			// 1. Send Response OK (Ack with started status)
			res := map[string]interface{}{
				"type": "res",
				"id":   req.ID,
				"ok":   true,
				"payload": map[string]interface{}{
					"runId":  runId,
					"status": "started",
				},
			}
			if err := ws.WriteJSON(res); err != nil {
				s.logger.Error().Err(err).Msg("Failed to write response")
				break
			}

			// 2. Call Agent Service in background (Streaming)
			go func() {
				var fullResponse strings.Builder
				seq := 0

				// Helper to send events
				sendEvent := func(state string, delta string, msgObj interface{}) {
					payload := map[string]interface{}{
						"runId":      runId,
						"sessionKey": sessionKey,
						"seq":        seq,
						"state":      state,
					}
					if delta != "" {
						payload["delta"] = map[string]interface{}{"text": delta}
					}
					if msgObj != nil {
						payload["message"] = msgObj
					}

					event := map[string]interface{}{
						"type":    "event",
						"event":   "chat",
						"payload": payload,
					}
					_ = ws.WriteJSON(event)
					seq++
				}

				err := s.agentService.ProcessChat(context.Background(), sessionKey, message, func(delta string) {
					fullResponse.WriteString(delta)

					// Client expects 'message' to find the text to display.
					// Controller logic sets state.chatStream = next (where next is extracted from message).
					// So we must send the FULL text so far, or at least a message structure.
					partialMessage := map[string]interface{}{
						"role": "assistant",
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": fullResponse.String(),
							},
						},
					}
					sendEvent("delta", delta, partialMessage)
				})

				if err != nil {
					s.logger.Error().Err(err).Msg("Agent processing failed")
					// Send error event
					// payload: { state: "error", errorMessage: ... }
					errPayload := map[string]interface{}{
						"runId":        runId,
						"sessionKey":   sessionKey,
						"seq":          seq,
						"state":        "error",
						"errorMessage": err.Error(),
					}
					_ = ws.WriteJSON(map[string]interface{}{
						"type":    "event",
						"event":   "chat",
						"payload": errPayload,
					})
					return
				}

				respStr := fullResponse.String()
				s.logger.Info().Str("response", respStr).Msg("Full Agent Response")

				// Store Assistant Message in Persistent History
				_ = s.sessionManager.AddMessage(sessionKey, "assistant", respStr)

				// Reconstruct asstMsg for final event
				asstMsg := map[string]interface{}{
					"role": "assistant",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": respStr,
						},
					},
					"timestamp": time.Now().UnixMilli(),
				}

				// Send "final" event
				sendEvent("final", "", asstMsg)
			}()

		default:
			// Unknown method response
			res := map[string]interface{}{
				"type":  "res",
				"id":    req.ID,
				"ok":    false,
				"error": map[string]interface{}{"code": "method_not_found", "message": "Method not found: " + req.Method},
			}
			_ = ws.WriteJSON(res)
		}
	}

	return nil
}
