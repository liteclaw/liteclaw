package gateway

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/liteclaw/liteclaw/internal/config"
)

// AuthMiddleware returns a middleware that validates the gateway token.
func (s *Server) AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Skip auth for health check
		if c.Path() == "/health" {
			return next(c)
		}

		cfg, err := config.Load()
		if err != nil || cfg.Gateway.Auth.Token == "" {
			// If no token is configured, allow for now (parities with TS default behavior if token empty)
			return next(c)
		}

		// Extract token
		token := extractToken(c.Request())
		if token == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "Missing authentication token")
		}

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.Gateway.Auth.Token)) != 1 {
			return echo.NewHTTPError(http.StatusUnauthorized, "Invalid authentication token")
		}

		return next(c)
	}
}

// HookAuthMiddleware returns a middleware that validates the hook-specific token.
func (s *Server) HookAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg, err := config.Load()
		if err != nil || !cfg.Hooks.Internal.Enabled || cfg.Hooks.Internal.Token == "" {
			// If hooks disabled or no token, use global auth or allow
			return next(c)
		}

		token := extractToken(c.Request())
		if token == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "Missing hook token")
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.Hooks.Internal.Token)) != 1 {
			return echo.NewHTTPError(http.StatusUnauthorized, "Invalid hook token")
		}

		return next(c)
	}
}

// RateLimitMiddleware returns a middleware that limits requests per IP.
func (s *Server) RateLimitMiddleware() echo.MiddlewareFunc {
	cfg, _ := config.Load()
	if cfg == nil || !cfg.Gateway.RateLimit.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	rps := cfg.Gateway.RateLimit.RPS
	if rps <= 0 {
		rps = 10 // Default RPS
	}
	burst := cfg.Gateway.RateLimit.Burst
	if burst <= 0 {
		burst = 20 // Default Burst
	}

	config := middleware.RateLimiterConfig{
		Skipper: middleware.DefaultSkipper,
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(rps),
				Burst:     burst,
				ExpiresIn: 0,
			},
		),
		IdentifierExtractor: func(ctx echo.Context) (string, error) {
			id := ctx.RealIP()
			return id, nil
		},
		ErrorHandler: func(context echo.Context, err error) error {
			return context.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "Too many requests",
			})
		},
		DenyHandler: func(context echo.Context, identifier string, err error) error {
			return context.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "Rate limit exceeded",
			})
		},
	}

	return middleware.RateLimiterWithConfig(config)
}

func extractToken(r *http.Request) string {
	// 1. Authorization: Bearer <token>
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}

	// 2. X-Clawdbot-Token
	if token := r.Header.Get("X-Clawdbot-Token"); token != "" {
		return token
	}

	// 3. Query parameter ?token=<token>
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}

	return ""
}
