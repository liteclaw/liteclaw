// Package test provides test utilities and helpers for LiteClaw tests.
package test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

// MockServer creates a mock HTTP server for testing.
type MockServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []RecordedRequest
	handlers map[string]http.HandlerFunc
}

// RecordedRequest represents a recorded HTTP request.
type RecordedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

// NewMockServer creates a new mock server.
func NewMockServer() *MockServer {
	ms := &MockServer{
		requests: make([]RecordedRequest, 0),
		handlers: make(map[string]http.HandlerFunc),
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record request
		ms.mu.Lock()
		ms.requests = append(ms.requests, RecordedRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
		})
		ms.mu.Unlock()

		// Find handler
		key := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		if handler, ok := ms.handlers[key]; ok {
			handler(w, r)
			return
		}

		// Default 404
		w.WriteHeader(http.StatusNotFound)
	}))

	return ms
}

// HandleFunc registers a handler for a specific method and path.
func (ms *MockServer) HandleFunc(method, path string, handler http.HandlerFunc) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	key := fmt.Sprintf("%s %s", method, path)
	ms.handlers[key] = handler
}

// HandleJSON registers a handler that returns JSON.
func (ms *MockServer) HandleJSON(method, path string, statusCode int, body string) {
	ms.HandleFunc(method, path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	})
}

// Requests returns all recorded requests.
func (ms *MockServer) Requests() []RecordedRequest {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	result := make([]RecordedRequest, len(ms.requests))
	copy(result, ms.requests)
	return result
}

// Reset clears recorded requests.
func (ms *MockServer) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.requests = make([]RecordedRequest, 0)
}

// GetFreePort returns a free port for testing.
func GetFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	return listener.Addr().(*net.TCPAddr).Port
}

// WaitForPort waits for a port to become available.
func WaitForPort(t *testing.T, host string, port int, timeout time.Duration) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}

	return false
}

// Poll polls a function until it returns true or times out.
func Poll(t *testing.T, ctx context.Context, interval time.Duration, fn func() bool) bool {
	t.Helper()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if fn() {
				return true
			}
		}
	}
}
