// Package channels provides the communication channel framework.
package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Registry manages all channel adapters.
// It provides a unified entry point for the Gateway to interact with all messaging platforms.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
	handler  MessageHandler
	logger   *zerolog.Logger
}

// NewRegistry creates a new channel registry.
func NewRegistry(logger *zerolog.Logger, handler MessageHandler) *Registry {
	return &Registry{
		adapters: make(map[string]Adapter),
		handler:  handler,
		logger:   logger,
	}
}

// Register adds an adapter to the registry.
func (r *Registry) Register(adapter Adapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := adapter.ID()
	if _, exists := r.adapters[id]; exists {
		return fmt.Errorf("adapter %q already registered", id)
	}

	// Set the handler so the adapter can route messages to Gateway
	adapter.SetHandler(r.handler)

	r.adapters[id] = adapter
	r.logger.Info().
		Str("adapter", id).
		Str("type", string(adapter.Type())).
		Msg("Channel adapter registered")

	return nil
}

// Unregister removes an adapter from the registry.
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	adapter, exists := r.adapters[id]
	if !exists {
		return fmt.Errorf("adapter %q not found", id)
	}

	// Stop if running
	if adapter.IsRunning() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = adapter.Stop(ctx)
	}

	delete(r.adapters, id)
	r.logger.Info().Str("adapter", id).Msg("Channel adapter unregistered")

	return nil
}

// Get returns an adapter by ID.
func (r *Registry) Get(id string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[id]
	return adapter, ok
}

// GetByType returns all adapters of a specific type.
func (r *Registry) GetByType(chanType ChannelType) []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Adapter
	for _, adapter := range r.adapters {
		if adapter.Type() == chanType {
			result = append(result, adapter)
		}
	}
	return result
}

// All returns all registered adapters.
func (r *Registry) All() []Adapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Adapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		result = append(result, adapter)
	}
	return result
}

// StartAll starts all registered adapters.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	adapters := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		adapters = append(adapters, a)
	}
	r.mu.RUnlock()

	for _, adapter := range adapters {
		if err := adapter.Start(ctx); err != nil {
			r.logger.Error().
				Err(err).
				Str("adapter", adapter.ID()).
				Msg("Failed to start adapter")
			// Continue trying to start others
		} else {
			r.logger.Info().
				Str("adapter", adapter.ID()).
				Msg("Adapter started")
		}
	}

	return nil
}

// StopAll stops all registered adapters.
func (r *Registry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	adapters := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		adapters = append(adapters, a)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, adapter := range adapters {
		if err := adapter.Stop(ctx); err != nil {
			r.logger.Error().
				Err(err).
				Str("adapter", adapter.ID()).
				Msg("Failed to stop adapter")
			lastErr = err
		}
	}

	return lastErr
}

// Send routes a message to the appropriate adapter.
func (r *Registry) Send(ctx context.Context, req *SendRequest) (*SendResult, error) {
	adapter, ok := r.Get(string(req.To.ChannelType))
	if !ok {
		// Try to find by type
		adapters := r.GetByType(ChannelType(req.To.ChannelType))
		if len(adapters) == 0 {
			return nil, fmt.Errorf("no adapter found for channel type %q", req.To.ChannelType)
		}
		adapter = adapters[0]
	}

	return adapter.Send(ctx, req)
}

// Status returns the status of all adapters.
func (r *Registry) Status() []AdapterStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var statuses []AdapterStatus
	for _, adapter := range r.adapters {
		state := adapter.State()
		statuses = append(statuses, AdapterStatus{
			ID:        adapter.ID(),
			Name:      adapter.Name(),
			Type:      adapter.Type(),
			Running:   state.Running,
			Mode:      state.Mode,
			LastError: state.LastError,
		})
	}

	return statuses
}

// AdapterStatus represents an adapter's status for API responses.
type AdapterStatus struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      ChannelType `json:"type"`
	Running   bool        `json:"running"`
	Mode      string      `json:"mode,omitempty"`
	LastError string      `json:"lastError,omitempty"`
}
