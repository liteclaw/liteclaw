// Package gateway provides the LiteClaw gateway server.
// This is the core control plane that manages channels, agents, and sessions.
package gateway

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"golang.org/x/term"

	"github.com/liteclaw/liteclaw/extensions/dingtalk"
	"github.com/liteclaw/liteclaw/extensions/discord"
	"github.com/liteclaw/liteclaw/extensions/feishu"
	"github.com/liteclaw/liteclaw/extensions/imessage"
	"github.com/liteclaw/liteclaw/extensions/qq"
	"github.com/liteclaw/liteclaw/extensions/telegram"
	"github.com/liteclaw/liteclaw/extensions/wecom"
	"github.com/liteclaw/liteclaw/internal/agent"
	"github.com/liteclaw/liteclaw/internal/browser"
	"github.com/liteclaw/liteclaw/internal/channels"
	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/pairing"
)

// Config holds the gateway configuration.
type Config struct {
	Host string
	Port int
}

// Server represents the LiteClaw gateway server.
type Server struct {
	config *Config
	echo   *echo.Echo
	logger zerolog.Logger

	// Runtime state
	mu        sync.RWMutex
	running   bool
	startTime time.Time

	// Managers
	// channelManager *channels.Manager   // TODO
	// sessionManager *session.Manager    	// Simple in-memory history: sessionKey -> list of message objects
	// Services
	agentService   *agent.Service
	sessionManager *SessionManager
	relayManager   *browser.RelayManager
	adapters       map[string]channels.Adapter

	// Dedicated servers to shutdown
	shutdownServers []*echo.Echo
}

// New creates a new gateway server.
func New(cfg *Config) *Server {
	// Setup logger
	// Use standard JSON logger to avoid terminal compatibility issues with ConsoleWriter
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("component", "gateway").Logger()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = NewCustomValidator()

	return &Server{
		config:         cfg,
		echo:           e,
		logger:         logger,
		adapters:       make(map[string]channels.Adapter),
		sessionManager: NewSessionManager(""),
		relayManager:   browser.NewRelayManager(),
	}
}

// Start starts the gateway server.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("gateway already running")
	}
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to load config, using defaults")
		// Use empty or default config if load fails
		ctxCfg := &config.Config{Env: map[string]string{}}
		s.agentService = agent.NewService(ctxCfg, s)
	} else {
		s.logger.Info().Msg("Configuration loaded")
		s.agentService = agent.NewService(cfg, s)

		// Initialize Telegram Adapter if configured
		if cfg.Channels.Telegram.BotToken != "" {
			tgCfg := &telegram.Config{
				Token: cfg.Channels.Telegram.BotToken,
			}
			tgAdapter := telegram.New(tgCfg, s.logger)

			// Set Handler
			handler := NewChannelHandler(s)
			tgAdapter.SetHandler(handler)

			// Register and Start
			s.adapters["telegram"] = tgAdapter

			// Start in background
			go func() {
				if err := tgAdapter.Start(context.Background()); err != nil {
					s.logger.Error().Err(err).Msg("Failed to start Telegram adapter")
				}
			}()
		}

		// Initialize Discord Adapter if configured
		if cfg.Channels.Discord.Token != "" {
			// Calculate Intents Bitmask
			intents := 0
			if cfg.Channels.Discord.Intents.Presence {
				// GatewayIntentGuildPresences = 1 << 8 (256)
				intents |= (1 << 8)
			}
			if cfg.Channels.Discord.Intents.GuildMembers {
				// GatewayIntentGuildMembers = 1 << 1 (2)
				intents |= (1 << 1)
			}
			// Always add Message Content if we want to read messages (1 << 15)
			// But usually library handles defaults. Let's start with basic + configured.
			// Actually discord/adapter.go has DefaultIntents constant.
			// We can OR them.

			// Map Guilds Configuration
			guilds := make(map[string]discord.GuildConfig)
			for id, g := range cfg.Channels.Discord.Guilds {
				channels := make(map[string]discord.ChannelConfig)
				for cid, c := range g.Channels {
					channels[cid] = discord.ChannelConfig{
						Allow:   c.Allow,
						Enabled: c.Enabled,
					}
				}
				guilds[id] = discord.GuildConfig{
					Slug:     g.Slug,
					Channels: channels,
				}
			}

			dsCfg := &discord.Config{
				Token:       cfg.Channels.Discord.Token,
				Intents:     intents,
				GroupPolicy: cfg.Channels.Discord.GroupPolicy,
				Guilds:      guilds,
			}
			// Use default intents if 0, otherwise merge with defaults?
			// The adapter logic was: if dsCfg.Intents == 0 { dsCfg.Intents = discord.DefaultIntents }
			// Let's keep that behavior. If user sets specific intents, we use those + defaults?
			// For simplicity: If user provides NO special intents (0), we use DefaultIntents.
			// If user provides special intents, we should probably OR them with defaults
			// or just assume they know what they are doing.
			// But for now, let's just pass the computed intents.
			if dsCfg.Intents == 0 {
				dsCfg.Intents = discord.DefaultIntents
			} else {
				// Ensure defaults are present even if extras are added
				dsCfg.Intents |= discord.DefaultIntents
			}

			dsAdapter := discord.New(dsCfg, s.logger)

			// Set Handler (reuse same handler)
			handler := NewChannelHandler(s)
			dsAdapter.SetHandler(handler)

			// Register and Start
			s.adapters["discord"] = dsAdapter

			// Start in background
			go func() {
				if err := dsAdapter.Start(context.Background()); err != nil {
					s.logger.Error().Err(err).Msg("Failed to start Discord adapter")
				}
			}()
		}

		// Initialize iMessage Adapter if configured
		if cfg.Channels.IMessage.Enabled {
			imCfg := &imessage.Config{
				Enabled: true,
				DBPath:  cfg.Channels.IMessage.DBPath,
			}
			imAdapter := imessage.New(imCfg, s.logger)

			// Set Handler (reuse same handler)
			handler := NewChannelHandler(s)
			imAdapter.SetHandler(handler)

			// Register and Start
			s.adapters["imessage"] = imAdapter

			// Start in background
			go func() {
				if err := imAdapter.Start(context.Background()); err != nil {
					s.logger.Error().Err(err).Msg("Failed to start iMessage adapter")
				}
			}()
		}

		// Initialize QQ Adapter if configured
		if cfg.Channels.QQ.Enabled {
			qqCfg := &qq.Config{
				AppID:     cfg.Channels.QQ.AppID,
				AppSecret: cfg.Channels.QQ.AppSecret,
				Sandbox:   cfg.Channels.QQ.Sandbox,
			}
			qqAdapter := qq.New(qqCfg, s.logger)

			// Handler & Register
			handler := NewChannelHandler(s)
			qqAdapter.SetHandler(handler)
			s.adapters["qq"] = qqAdapter

			go func() {
				if err := qqAdapter.Start(context.Background()); err != nil {
					s.logger.Error().Err(err).Msg("Failed to start QQ adapter")
				}
			}()
		}

		// Initialize Feishu Adapter if configured
		if cfg.Channels.Feishu.Enabled {
			fsCfg := &feishu.Config{
				AppID:     cfg.Channels.Feishu.AppID,
				AppSecret: cfg.Channels.Feishu.AppSecret,
			}
			fsAdapter := feishu.New(fsCfg, s.logger)

			// Handler & Register
			handler := NewChannelHandler(s)
			fsAdapter.SetHandler(handler)
			s.adapters["feishu"] = fsAdapter

			go func() {
				if err := fsAdapter.Start(context.Background()); err != nil {
					s.logger.Error().Err(err).Msg("Failed to start Feishu adapter")
				}
			}()
		}

		// Initialize DingTalk Adapter if configured
		if cfg.Channels.DingTalk.Enabled {
			dtCfg := &dingtalk.Config{
				AppKey:    cfg.Channels.DingTalk.AppKey,
				AppSecret: cfg.Channels.DingTalk.AppSecret,
			}
			dtAdapter := dingtalk.New(dtCfg, s.logger)

			// Handler & Register
			handler := NewChannelHandler(s)
			dtAdapter.SetHandler(handler)
			s.adapters["dingtalk"] = dtAdapter

			go func() {
				if err := dtAdapter.Start(context.Background()); err != nil {
					s.logger.Error().Err(err).Msg("Failed to start DingTalk adapter")
				}
			}()
		}

		// Initialize WeCom Adapter if configured
		if cfg.Channels.WeCom.Enabled {
			wcCfg := &wecom.Config{
				Token:          cfg.Channels.WeCom.Token,
				EncodingAESKey: cfg.Channels.WeCom.EncodingAESKey,
				Port:           cfg.Channels.WeCom.Port,
				BotID:          cfg.Channels.WeCom.BotID,
			}
			wcAdapter := wecom.New(wcCfg, s.logger)

			// Set Handler
			handler := NewChannelHandler(s)
			wcAdapter.SetHandler(handler)
			s.adapters["wecom"] = wcAdapter

			// Start WeCom Adapter in background
			go func() {
				if err := wcAdapter.Start(context.Background()); err != nil {
					s.logger.Error().Err(err).Msg("Failed to start WeCom adapter")
				}
			}()

			// Create dedicated server for WeCom if port is specified
			if wcCfg.Port > 0 {
				wecomServer := echo.New()
				wecomServer.HideBanner = true
				wecomServer.HidePort = true

				// Register Handlers
				// WeCom adapter's HandleWebhook method is compatible with echo.HandlerFunc
				wecomServer.Any("/wecom", wcAdapter.HandleWebhook)

				// Start dedicated server on 0.0.0.0 (WeCom callbacks require public access)
				go func() {
					addr := fmt.Sprintf("0.0.0.0:%d", wcCfg.Port)
					s.logger.Info().Str("addr", addr).Msg("Starting WeCom callback server")
					if err := wecomServer.Start(addr); err != nil && err != http.ErrServerClosed {
						s.logger.Error().Err(err).Msg("WeCom server failed")
					}
				}()

				// Add to shutdown list
				s.shutdownServers = append(s.shutdownServers, wecomServer)
			} else {
				// Fallback to main server if no port specified (legacy behavior)
				s.echo.Any("/wecom", wcAdapter.HandleWebhook)
			}
		}
	}

	// Setup middleware
	s.setupMiddleware()

	// Setup routes
	s.setupRoutes()

	// Create server address
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	// Start main gateway server
	go func() {
		s.logger.Info().Str("addr", addr).Msg("Gateway server starting")
		if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
			s.logger.Fatal().Err(err).Msg("Gateway server failed")
		}
	}()

	// Start dedicated Browser Control Server (18791)
	go func() {
		controlAddr := fmt.Sprintf("%s:%d", s.config.Host, 18791)
		s.logger.Info().Str("addr", controlAddr).Msg("Browser Control server starting")

		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.CORS())

		// Map routes to existing handlers
		e.GET("/tabs", s.handleBrowserTabs)
		e.POST("/tabs/open", s.handleBrowserOpenTab)
		e.POST("/tabs/focus", s.handleBrowserFocusTab)
		e.DELETE("/tabs/:id", s.handleBrowserCloseTab)
		e.POST("/navigate", s.handleBrowserNavigate)
		e.GET("/snapshot", s.handleBrowserSnapshot)
		e.POST("/screenshot", s.handleBrowserScreenshot)
		e.POST("/act", s.handleBrowserAct)
		e.GET("/profiles", s.handleBrowserProfiles)
		e.GET("/", s.handleBrowserStatus)

		if err := e.Start(controlAddr); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("Browser Control server failed")
		}
	}()

	// Start dedicated Extension Relay Server (18792)
	go func() {
		relayAddr := fmt.Sprintf("%s:%d", s.config.Host, 18792)
		s.logger.Info().Str("addr", relayAddr).Msg("Extension Relay server starting")

		e := echo.New()
		e.HideBanner = true

		// Use a more permissive CORS for the extension
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "HEAD", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		}))

		// WebSocket relay
		e.GET("/extension", s.handleExtensionRelay)

		// HTTP Health Check / Status for the options page
		statusHandler := func(c echo.Context) error {
			if strings.Contains(c.Request().Header.Get("Upgrade"), "websocket") {
				return s.handleExtensionRelay(c)
			}
			if c.Request().Method == "HEAD" {
				return c.NoContent(http.StatusOK)
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":       true,
				"service":  "LiteClaw Browser Relay",
				"profiles": s.relayManager.ListProfiles(),
			})
		}

		e.GET("/", statusHandler)
		e.HEAD("/", statusHandler)

		// Compatibility endpoint
		e.GET("/extension/status", func(c echo.Context) error {
			profiles := s.relayManager.ListProfiles()
			return c.JSON(http.StatusOK, map[string]interface{}{
				"connected": len(profiles) > 0,
				"profiles":  profiles,
			})
		})

		if err := e.Start(relayAddr); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("Extension Relay server failed")
		}
	}()

	// Print startup message
	s.printStartupBanner()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Fallback: if terminal is in raw/no-ISIG mode, Ctrl+C may appear as byte 0x03.
	// Capture it so users can still stop the gateway.
	manualQuit := make(chan struct{}, 1)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		go func() {
			reader := bufio.NewReader(os.Stdin)
			for {
				b, err := reader.ReadByte()
				if err != nil {
					return
				}
				if b == 3 {
					manualQuit <- struct{}{}
					return
				}
			}
		}()
	}

	select {
	case <-quit:
	case <-manualQuit:
	}

	s.logger.Info().Msg("Shutting down gateway server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop adapters
	for _, adapter := range s.adapters {
		_ = adapter.Stop(ctx)
	}

	// Stop adapter
	// Agent service effectively stops when gateway stops receiving requests checking policies etc.

	// Stop auxiliary servers
	for _, srv := range s.shutdownServers {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := srv.Shutdown(ctx); err != nil {
			s.logger.Error().Err(err).Msg("Failed to shutdown auxiliary server")
		}
		cancel()
	}

	// Stop main server
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.echo.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info().Msg("Server stopped")
	return nil
}

// processChannelMessage handles incoming messages from channels
func (s *Server) processChannelMessage(ctx context.Context, msg *channels.IncomingMessage) error {
	s.logger.Info().
		Str("channel", msg.ChannelType).
		Str("sender", msg.SenderName).
		Str("text", msg.Text).
		Msg("Processing channel message")

	adapter, ok := s.adapters[msg.ChannelType]
	if !ok {
		s.logger.Warn().Str("channel", msg.ChannelType).Msg("Adapter not found for channel")
		return nil
	}

	// Check DMPolicy (Secure Pairing)
	dmPolicy := s.getDMPolicy(msg.ChannelType)
	if dmPolicy == "pairing" && msg.ChatType == string(channels.ChatTypeDirect) {
		allowed, err := pairing.IsAllowed(msg.ChannelType, msg.SenderID)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to check pairing status")
			// Fail open or closed? Closed seems safer for "pairing" policy.
			return nil
		}

		if !allowed {
			s.logger.Info().Str("sender", msg.SenderID).Msg("Sender not allowed by pairing policy. Upserting request.")

			code, created, err := pairing.UpsertChannelPairingRequest(msg.ChannelType, msg.SenderID, map[string]string{
				"senderName": msg.SenderName,
				"text":       msg.Text,
			})

			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to upsert pairing request")
				return nil
			}

			// Reply with instructions
			var text string
			if created {
				text = fmt.Sprintf("🔐 **Secure Pairing Required**\n\nYour pairing code is: `%s`\n\nAsk the administrator to approve it.", code)
			} else {
				text = fmt.Sprintf("🔐 **Secure Pairing Required**\n\nYour request is pending (Code: `%s`).\nTo approve, ask admin to run:\n`liteclaw pairing approve --channel %s %s`", code, msg.ChannelType, code)
			}

			_, err = adapter.Send(ctx, &channels.SendRequest{
				To:      channels.Destination{ChatID: msg.ChatID},
				Text:    text,
				ReplyTo: msg.ID,
			})
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to send pairing instructions")
			}
			return nil // Stop processing
		}
	}

	// Use SenderID as session key for simple persistence/context
	sessionKey := fmt.Sprintf("%s:%s", msg.ChannelType, msg.SenderID)

	// Load persisted history into agent session (restore context after gateway restart)
	// Optimization: Check if agent already has session in memory to avoid parsing disk history on every message
	if !s.agentService.HasSession(sessionKey) {
		if history, err := s.sessionManager.GetHistory(sessionKey); err == nil && len(history) > 0 {
			// Convert gateway.Message to agent.Message format
			agentHistory := make([]agent.Message, 0, len(history))
			for _, m := range history {
				// Extract text from Content array
				text := ""
				if len(m.Content) > 0 {
					if t, ok := m.Content[0]["text"].(string); ok {
						text = t
					}
				}
				if text != "" {
					agentHistory = append(agentHistory, agent.Message{
						Role:    m.Role,
						Content: text,
					})
				}
			}
			s.agentService.LoadSessionHistory(sessionKey, agentHistory)
		}
	}

	// Persist User Message
	if err := s.sessionManager.AddMessage(sessionKey, "user", msg.Text); err != nil {
		s.logger.Warn().Err(err).Str("session", sessionKey).Msg("Failed to persist user message")
	}

	var fullResponse strings.Builder
	// TUI Streaming Effect: print to stdout
	fmt.Printf("\n>>> Streaming Response for %s:\n", sessionKey)
	err := s.agentService.ProcessChat(ctx, sessionKey, msg.Text, func(delta string) {
		fmt.Print(delta)
		fullResponse.WriteString(delta)
	})
	fmt.Println("\n<<< End Stream")

	if err != nil {
		s.logger.Error().Err(err).Msg("Agent processing failed")
		return err
	}

	respStr := fullResponse.String()

	// Clean up response: remove <think>...</think> blocks and trim whitespace
	// This is done here so the clean response is logged and sent
	thinkRegex := regexp.MustCompile(`(?s)<think>.*?</think>`)
	respStr = thinkRegex.ReplaceAllString(respStr, "")
	respStr = strings.TrimSpace(respStr)

	s.logger.Info().Str("response", respStr).Msg("Full Agent Response")

	// Persist Assistant Response
	if err := s.sessionManager.AddMessage(sessionKey, "assistant", respStr); err != nil {
		s.logger.Warn().Err(err).Str("session", sessionKey).Msg("Failed to persist assistant message")
	}

	// Send Reply
	_, err = adapter.Send(ctx, &channels.SendRequest{
		To:      channels.Destination{ChatID: msg.ChatID},
		Text:    respStr,
		ReplyTo: msg.ID,
	})

	return err
}

// SendMessage implements tools.MessageSender to allow agents to send messages via the gateway.
func (s *Server) SendMessage(ctx context.Context, channel, target, message string) error {
	s.logger.Info().Str("channel", channel).Str("target", target).Msg("Agent requested message send")

	// If implicit targeting (empty target), try to resolve from session history
	if target == "" {
		resolvedChannel, resolvedTarget, found := s.sessionManager.ResolveDeliveryTarget()
		if found {
			s.logger.Info().Str("channel", resolvedChannel).Str("target", resolvedTarget).Msg("Resolved implicit delivery target")
			channel = resolvedChannel
			target = resolvedTarget
		} else {
			return fmt.Errorf("target is required and could not be resolved from session history")
		}
	}

	adapter, ok := s.adapters[channel]
	if !ok {
		return fmt.Errorf("adapter for channel '%s' not found or not enabled", channel)
	}

	_, err := adapter.Send(ctx, &channels.SendRequest{
		To:   channels.Destination{ChatID: target},
		Text: message,
	})
	return err
}

// setupMiddleware configures Echo middleware.
func (s *Server) setupMiddleware() {
	// Request logging
	s.echo.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			s.logger.Info().
				Str("method", v.Method).
				Str("uri", v.URI).
				Int("status", v.Status).
				Msg("request")
			return nil
		},
	}))

	// Recover from panics
	s.echo.Use(middleware.Recover())

	// Rate Limiting (Global)
	s.echo.Use(s.RateLimitMiddleware())

	// CORS
	s.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
	}))
}

// setupRoutes configures HTTP routes.
func (s *Server) setupRoutes() {
	// Health check
	s.echo.GET("/health", s.handleHealth)

	// Static Assets (Vite build output)
	s.echo.Static("/assets", "dist/control-ui/assets")
	s.echo.File("/favicon.ico", "dist/control-ui/favicon.ico")

	// Root Handler: WebSocket Upgrade or Serve Web UI
	s.echo.GET("/", func(c echo.Context) error {
		if strings.ToLower(c.Request().Header.Get("Upgrade")) == "websocket" {
			// Apply auth to websocket upgrades
			return s.AuthMiddleware(s.handleWebSocket)(c)
		}
		return c.File("dist/control-ui/index.html")
	})

	// API routes
	api := s.echo.Group("/api")
	api.Use(s.AuthMiddleware)
	{
		// Status
		api.GET("/status", s.handleStatus)

		// Sessions
		api.GET("/sessions", s.handleListSessions)
		api.POST("/sessions/:id/send", s.handleSendToSession)

		// Channels
		api.GET("/channels", s.handleListChannels)

		// Config
		api.GET("/config", s.handleGetConfig)
		api.POST("/config", s.handleUpdateConfig)

		// Cron
		api.GET("/cron/jobs", s.handleCronList)
		api.POST("/cron/jobs", s.handleCronAdd)
		api.GET("/cron/jobs/:id", s.handleCronGet)
		api.DELETE("/cron/jobs/:id", s.handleCronRemove)
		api.POST("/cron/jobs/:id/update", s.handleCronUpdate)
		api.POST("/cron/jobs/:id/run", s.handleCronRun)
		api.GET("/cron/jobs/:id/history", s.handleCronHistory)

		// Gateway control
		api.POST("/gateway/restart", s.handleRestart)
		api.POST("/gateway/reload", s.handleReload)

		// Browser Control API (Relay)
		api.GET("/browser/tabs", s.handleBrowserTabs)
		api.POST("/browser/tabs/open", s.handleBrowserOpenTab)
		api.POST("/browser/tabs/focus", s.handleBrowserFocusTab)
		api.DELETE("/browser/tabs/:id", s.handleBrowserCloseTab)
		api.POST("/browser/navigate", s.handleBrowserNavigate)
		api.GET("/browser/snapshot", s.handleBrowserSnapshot)
		api.POST("/browser/screenshot", s.handleBrowserScreenshot)
		api.POST("/browser/act", s.handleBrowserAct)
		api.GET("/browser/profiles", s.handleBrowserProfiles)
		api.GET("/browser/", s.handleBrowserStatus)
	}

	// Browser Extension Relay WebSocket
	s.echo.GET("/extension/relay", s.HookAuthMiddleware(s.handleExtensionRelay))

	// SPA Fallback: Any other GET request not starting with /api returns index.html
	s.echo.GET("/*", func(c echo.Context) error {
		if strings.HasPrefix(c.Request().URL.Path, "/api") {
			return echo.ErrNotFound
		}
		return c.File("dist/control-ui/index.html")
	})
}

func (s *Server) printStartupBanner() {
	fmt.Println()
	fmt.Println("  🦞 LiteClaw Gateway")
	fmt.Println("  =================")
	fmt.Printf("  ✓ HTTP server listening on http://%s:%d\n", s.config.Host, s.config.Port)
	fmt.Printf("  ✓ WebSocket endpoint: ws://%s:%d\n", s.config.Host, s.config.Port)
	fmt.Println()
	fmt.Println("  Browser Extension Support")
	fmt.Println("  -------------------------")
	fmt.Printf("  ✓ Control API: http://%s:18791\n", s.config.Host)
	fmt.Printf("  ✓ Extension Relay: ws://%s:18792\n", s.config.Host)
	fmt.Println("  (Check logs for '[Relay] COMMAND SENT' and 'Raw message' to debug)")
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println()
}

// IsRunning returns whether the gateway is running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Uptime returns how long the gateway has been running.
func (s *Server) Uptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.running {
		return 0
	}
	return time.Since(s.startTime)
}
func (s *Server) getDMPolicy(channelType string) string {
	// If agent service isn't ready, default to restrictive or empty?
	// Empty usually means open or default policy which is safer for startup
	if s.agentService == nil || s.agentService.Config == nil {
		return ""
	}

	switch channelType {
	case "telegram":
		return s.agentService.Config.Channels.Telegram.DMPolicy
	case "discord":
		return s.agentService.Config.Channels.Discord.DMPolicy
	// TODO: Add other channels when they support DMPolicy in config
	default:
		return ""
	}
}
