package gateway

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// handleExtensionRelay handles the WebSocket connection from the browser extension.
func (s *Server) handleExtensionRelay(c echo.Context) error {
	profile := c.QueryParam("profile")
	if profile == "" {
		profile = "chrome" // Default profile
	}

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to upgrade extension relay connection")
		return err
	}
	defer ws.Close()

	s.logger.Info().Str("profile", profile).Msg("Browser extension connected")
	s.relayManager.RegisterConnection(profile, ws)
	defer s.relayManager.UnregisterConnection(profile)

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			s.logger.Info().Str("profile", profile).Msg("Browser extension disconnected")
			break
		}

		if err := s.relayManager.HandleResponse(profile, message); err != nil {
			s.logger.Error().Err(err).Msg("Failed to handle extension response")
		}
	}

	return nil
}

// Browser Control API Handlers

func (s *Server) handleBrowserTabs(c echo.Context) error {
	profile := c.QueryParam("profile")
	result, err := s.relayManager.Call(c.Request().Context(), profile, "tabs", nil, "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserOpenTab(c echo.Context) error {
	profile := c.QueryParam("profile")
	var params map[string]interface{}
	if err := c.Bind(&params); err != nil {
		return err
	}
	result, err := s.relayManager.Call(c.Request().Context(), profile, "tabs.open", params, "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserFocusTab(c echo.Context) error {
	profile := c.QueryParam("profile")
	var params map[string]interface{}
	if err := c.Bind(&params); err != nil {
		return err
	}
	targetID, _ := params["targetId"].(string)
	result, err := s.relayManager.Call(c.Request().Context(), profile, "tabs.focus", params, targetID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserCloseTab(c echo.Context) error {
	profile := c.QueryParam("profile")
	id := c.Param("id")
	result, err := s.relayManager.Call(c.Request().Context(), profile, "tabs.close", nil, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserNavigate(c echo.Context) error {
	profile := c.QueryParam("profile")
	var params map[string]interface{}
	if err := c.Bind(&params); err != nil {
		return err
	}
	targetID, _ := params["targetId"].(string)
	result, err := s.relayManager.Call(c.Request().Context(), profile, "navigate", params, targetID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserSnapshot(c echo.Context) error {
	profile := c.QueryParam("profile")
	targetID := c.QueryParam("targetId")
	result, err := s.relayManager.Call(c.Request().Context(), profile, "snapshot", nil, targetID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserScreenshot(c echo.Context) error {
	profile := c.QueryParam("profile")
	var params map[string]interface{}
	if err := c.Bind(&params); err != nil {
		return err
	}
	targetID, _ := params["targetId"].(string)
	result, err := s.relayManager.Call(c.Request().Context(), profile, "screenshot", params, targetID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserAct(c echo.Context) error {
	profile := c.QueryParam("profile")
	var params map[string]interface{}
	if err := c.Bind(&params); err != nil {
		return err
	}
	targetID, _ := params["targetId"].(string)
	result, err := s.relayManager.Call(c.Request().Context(), profile, "act", params, targetID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleBrowserProfiles(c echo.Context) error {
	profiles := s.relayManager.ListProfiles()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"profiles": profiles,
	})
}

func (s *Server) handleBrowserStatus(c echo.Context) error {
	profiles := s.relayManager.ListProfiles()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":       true,
		"profiles": profiles,
		"service":  "LiteClaw Browser Relay",
	})
}
