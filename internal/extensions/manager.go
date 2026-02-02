// Package extensions provides the plugin/extension system for LiteClaw.
// Extensions allow adding new channels, tools, and capabilities dynamically.
package extensions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"

	"github.com/rs/zerolog"

	"github.com/liteclaw/liteclaw/internal/channels"
)

// Extension represents a loaded extension.
type Extension interface {
	// ID returns the unique extension identifier.
	ID() string
	// Name returns the human-readable name.
	Name() string
	// Description returns the extension description.
	Description() string
	// Version returns the extension version.
	Version() string
	// Register registers the extension with the API.
	Register(api *API) error
	// Unregister cleans up the extension.
	Unregister() error
}

// API provides the interface for extensions to interact with LiteClaw.
type API struct {
	logger         *zerolog.Logger
	channelManager *channels.Manager
	registry       *channels.Registry

	mu       sync.RWMutex
	channels []channels.Channel
	adapters []channels.Adapter
	tools    []ToolRegistration
}

// ToolRegistration represents a tool registered by an extension.
type ToolRegistration struct {
	ExtensionID string
	Name        string
	Description string
	Handler     interface{}
}

// NewAPI creates a new extension API.
func NewAPI(logger zerolog.Logger, channelManager *channels.Manager) *API {
	return &API{
		logger:         &logger,
		channelManager: channelManager,
		channels:       make([]channels.Channel, 0),
		adapters:       make([]channels.Adapter, 0),
		tools:          make([]ToolRegistration, 0),
	}
}

// RegisterChannel registers a channel from an extension (legacy).
func (api *API) RegisterChannel(ch channels.Channel) {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.channels = append(api.channels, ch)
	if api.channelManager != nil {
		api.channelManager.Register(ch)
	}
	api.logger.Info().Str("channel", ch.Name()).Msg("Extension registered channel")
}

// RegisterAdapter registers a channel adapter from an extension.
func (api *API) RegisterAdapter(adapter channels.Adapter) {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.adapters = append(api.adapters, adapter)
	if api.registry != nil {
		_ = api.registry.Register(adapter)
	}
	api.logger.Info().Str("adapter", adapter.ID()).Msg("Extension registered adapter")
}

// RegisterTool registers a tool from an extension.
func (api *API) RegisterTool(extensionID, name, description string, handler interface{}) {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.tools = append(api.tools, ToolRegistration{
		ExtensionID: extensionID,
		Name:        name,
		Description: description,
		Handler:     handler,
	})
	api.logger.Info().Str("tool", name).Str("extension", extensionID).Msg("Extension registered tool")
}

// Logger returns the logger for extensions to use.
func (api *API) Logger() *zerolog.Logger {
	return api.logger
}

// Manager manages extensions.
type Manager struct {
	logger     zerolog.Logger
	api        *API
	extensions map[string]Extension
	searchDirs []string

	mu sync.RWMutex
}

// NewManager creates a new extension manager.
func NewManager(logger zerolog.Logger, api *API) *Manager {
	return &Manager{
		logger:     logger.With().Str("component", "extensions").Logger(),
		api:        api,
		extensions: make(map[string]Extension),
		searchDirs: []string{},
	}
}

// AddSearchDir adds a directory to search for extensions.
func (m *Manager) AddSearchDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.searchDirs = append(m.searchDirs, dir)
}

// LoadAll loads all extensions from search directories.
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, dir := range m.searchDirs {
		if err := m.loadFromDir(dir); err != nil {
			m.logger.Warn().Err(err).Str("dir", dir).Msg("Failed to load extensions from directory")
		}
	}

	return nil
}

// loadFromDir loads extensions from a directory.
func (m *Manager) loadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		extDir := filepath.Join(dir, entry.Name())
		if err := m.loadExtension(extDir); err != nil {
			m.logger.Warn().Err(err).Str("dir", extDir).Msg("Failed to load extension")
		}
	}

	return nil
}

// loadExtension loads a single extension from a directory.
func (m *Manager) loadExtension(dir string) error {
	// Look for a plugin file (.so on Linux, .dylib on macOS)
	pluginPath := filepath.Join(dir, "plugin.so")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		pluginPath = filepath.Join(dir, "plugin.dylib")
	}

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		// No compiled plugin, try to load as built-in
		return nil
	}

	// Load the Go plugin
	p, err := plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	// Look for the Extension symbol
	sym, err := p.Lookup("Extension")
	if err != nil {
		return fmt.Errorf("plugin missing Extension symbol: %w", err)
	}

	ext, ok := sym.(Extension)
	if !ok {
		return fmt.Errorf("Extension symbol has wrong type")
	}

	// Register the extension
	if err := ext.Register(m.api); err != nil {
		return fmt.Errorf("failed to register extension: %w", err)
	}

	m.extensions[ext.ID()] = ext
	m.logger.Info().Str("id", ext.ID()).Str("name", ext.Name()).Msg("Loaded extension")

	return nil
}

// Register registers a built-in extension.
func (m *Manager) Register(ext Extension) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.extensions[ext.ID()]; exists {
		return fmt.Errorf("extension %s already registered", ext.ID())
	}

	if err := ext.Register(m.api); err != nil {
		return err
	}

	m.extensions[ext.ID()] = ext
	m.logger.Info().Str("id", ext.ID()).Str("name", ext.Name()).Msg("Registered extension")

	return nil
}

// Get returns an extension by ID.
func (m *Manager) Get(id string) (Extension, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ext, ok := m.extensions[id]
	return ext, ok
}

// List returns all registered extensions.
func (m *Manager) List() []Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Extension, 0, len(m.extensions))
	for _, ext := range m.extensions {
		result = append(result, ext)
	}
	return result
}

// UnloadAll unloads all extensions.
func (m *Manager) UnloadAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, ext := range m.extensions {
		if err := ext.Unregister(); err != nil {
			m.logger.Warn().Err(err).Str("id", id).Msg("Failed to unregister extension")
		}
	}

	m.extensions = make(map[string]Extension)
	return nil
}
