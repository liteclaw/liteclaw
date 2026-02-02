package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Default filenames for workspace bootstrap files
const (
	DefaultAgentsFilename    = "AGENTS.md"
	DefaultSoulFilename      = "SOUL.md"
	DefaultToolsFilename     = "TOOLS.md"
	DefaultIdentityFilename  = "IDENTITY.md"
	DefaultUserFilename      = "USER.md"
	DefaultHeartbeatFilename = "HEARTBEAT.md"
	DefaultBootstrapFilename = "BOOTSTRAP.md"
	DefaultMemoryFilename    = "MEMORY.md"
)

// BootstrapFile represents a loaded workspace file
type BootstrapFile struct {
	Name    string
	Path    string
	Content string
	Missing bool
}

// ResolveDefaultDir returns the default agent workspace directory
func ResolveDefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	profile := os.Getenv("LITECLAW_PROFILE")
	if strings.TrimSpace(profile) != "" && strings.ToLower(profile) != "default" {
		return filepath.Join(home, "clawd-"+profile)
	}
	return filepath.Join(home, "clawd")
}

// stripFrontMatter removes YAML frontmatter from content
func stripFrontMatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	// Find the second "---"
	// We look for "\n---" to ensure line break
	const marker = "\n---"
	endIndex := strings.Index(content[3:], marker)
	if endIndex == -1 {
		return content
	}
	// 3 (first ---) + endIndex + length of marker
	return strings.TrimSpace(content[3+endIndex+len(marker):])
}

// loadTemplate attempts to read a template file from the docs/reference/templates directory
func loadTemplate(filename string) (string, error) {
	// Try to locate templates relative to CWD (assuming dev environment)
	cwd, _ := os.Getwd()
	// Try a few possible locations
	candidates := []string{
		filepath.Join(cwd, "assets/templates"),         // New Standard Location
		filepath.Join(cwd, "docs/reference/templates"), // Legacy/Fallback
		"/opt/liteclaw/assets/templates",               // Production
	}

	for _, dir := range candidates {
		path := filepath.Join(dir, filename)
		if data, err := os.ReadFile(path); err == nil {
			return stripFrontMatter(string(data)), nil
		}
	}
	return "", fmt.Errorf("template not found: %s", filename)
}

// EnsureWorkspace ensures the workspace directory and default files exist.
func EnsureWorkspace(dir string) error {
	if dir == "" {
		dir = ResolveDefaultDir()
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace dir: %w", err)
	}

	// Define defaults, trying to load from templates first
	filenames := []string{
		DefaultAgentsFilename,
		DefaultSoulFilename,
		DefaultToolsFilename,
		DefaultIdentityFilename,
		DefaultUserFilename,
		DefaultHeartbeatFilename,
		DefaultBootstrapFilename,
	}

	for _, filename := range filenames {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Try to load from template
			content, err := loadTemplate(filename)
			if err != nil {
				// Fallback to minimal hardcoded defaults if template missing
				switch filename {
				case DefaultSoulFilename:
					content = "You are a helpful AI assistant. Be concise."
				case DefaultIdentityFilename:
					content = "Identity: LiteClaw Agent"
				case DefaultUserFilename:
					content = "User: Admin"
				case DefaultToolsFilename:
					content = "# Tools\n\nRefer to system prompt for available tools."
				case DefaultAgentsFilename:
					content = "# Agents\n\nNo sub-agents configured."
				case DefaultHeartbeatFilename:
					content = "HEARTBEAT_OK"
				case DefaultBootstrapFilename:
					content = "LiteClaw"
				default:
					continue
				}
				fmt.Printf("Warning: using fallback for %s (template not found)\n", filename)
			}

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				fmt.Printf("Warning: failed to create default %s: %v\n", filename, err)
			} else {
				fmt.Printf("Created %s from template/fallback\n", filename)
			}
		}
	}
	return nil
}

// LoadBootstrapFiles loads all standard configuration files from the workspace
func LoadBootstrapFiles(dir string) ([]BootstrapFile, error) {
	if dir == "" {
		dir = ResolveDefaultDir()
	}

	filenames := []string{
		DefaultAgentsFilename,
		DefaultSoulFilename,
		DefaultToolsFilename,
		DefaultIdentityFilename,
		DefaultUserFilename,
		DefaultHeartbeatFilename,
		DefaultBootstrapFilename,
		DefaultMemoryFilename,
	}

	var results []BootstrapFile

	for _, name := range filenames {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				results = append(results, BootstrapFile{
					Name:    name,
					Path:    path,
					Missing: true,
				})
			} else {
				return nil, fmt.Errorf("failed to read %s: %w", name, err)
			}
		} else {
			results = append(results, BootstrapFile{
				Name:    name,
				Path:    path,
				Content: string(content),
				Missing: false,
			})
		}
	}

	return results, nil
}
