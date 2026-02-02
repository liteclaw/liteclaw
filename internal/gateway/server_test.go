package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHandleHealth(t *testing.T) {
	server := New(&Config{Host: "localhost", Port: 3456})
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.handleHealth(c); err != nil {
		t.Fatalf("handleHealth() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("handleHealth() status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.Len() == 0 {
		t.Error("handleHealth() body is empty")
	}
}

func TestHandleRoot(t *testing.T) {
	server := New(&Config{Host: "localhost", Port: 3456})
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.handleRoot(c); err != nil {
		t.Fatalf("handleRoot() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("handleRoot() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServerNew(t *testing.T) {
	cfg := &Config{
		Host: "0.0.0.0",
		Port: 8080,
	}

	server := New(cfg)

	if server == nil {
		t.Fatal("New() returned nil")
	}

	if server.config.Host != cfg.Host {
		t.Errorf("New().config.Host = %s, want %s", server.config.Host, cfg.Host)
	}

	if server.config.Port != cfg.Port {
		t.Errorf("New().config.Port = %d, want %d", server.config.Port, cfg.Port)
	}

	if server.IsRunning() {
		t.Error("New() server should not be running")
	}
}
