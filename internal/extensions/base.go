// Package extensions provides the plugin/extension system for LiteClaw.
package extensions

// BaseExtension provides a base implementation for extensions.
type BaseExtension struct {
	id          string
	name        string
	description string
	version     string
}

// NewBaseExtension creates a new base extension.
func NewBaseExtension(id, name, description, version string) *BaseExtension {
	return &BaseExtension{
		id:          id,
		name:        name,
		description: description,
		version:     version,
	}
}

// ID returns the extension ID.
func (e *BaseExtension) ID() string {
	return e.id
}

// Name returns the extension name.
func (e *BaseExtension) Name() string {
	return e.name
}

// Description returns the extension description.
func (e *BaseExtension) Description() string {
	return e.description
}

// Version returns the extension version.
func (e *BaseExtension) Version() string {
	return e.version
}

// Register is a no-op for base extension.
func (e *BaseExtension) Register(api *API) error {
	return nil
}

// Unregister is a no-op for base extension.
func (e *BaseExtension) Unregister() error {
	return nil
}
