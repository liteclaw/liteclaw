// Package skills provides the skills system for LiteClaw.
// Skills are markdown files with YAML frontmatter that extend agent capabilities.
package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a loaded skill.
type Skill struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Homepage    string            `yaml:"homepage" json:"homepage,omitempty"`
	Location    string            `json:"location"`
	Content     string            `json:"content,omitempty"`
	Metadata    *ClawdbotMetadata `json:"metadata,omitempty"`
}

// ClawdbotMetadata holds Clawdbot-specific skill metadata.
type ClawdbotMetadata struct {
	Emoji    string         `yaml:"emoji" json:"emoji,omitempty"`
	Requires *SkillRequires `yaml:"requires" json:"requires,omitempty"`
	Install  []InstallSpec  `yaml:"install" json:"install,omitempty"`
	OS       []string       `yaml:"os" json:"os,omitempty"`
}

// SkillRequires defines requirements for a skill to be active.
type SkillRequires struct {
	Bins []string `yaml:"bins" json:"bins,omitempty"`
	Env  []string `yaml:"env" json:"env,omitempty"`
}

// InstallSpec defines how to install a required dependency.
type InstallSpec struct {
	ID      string   `yaml:"id" json:"id"`
	Kind    string   `yaml:"kind" json:"kind"`
	Formula string   `yaml:"formula" json:"formula,omitempty"`
	Bins    []string `yaml:"bins" json:"bins,omitempty"`
	Label   string   `yaml:"label" json:"label"`
}

// Loader loads skills from directories.
type Loader struct {
	dirs []string
}

// NewLoader creates a new skill loader.
func NewLoader(dirs ...string) *Loader {
	return &Loader{dirs: dirs}
}

// LoadAll loads all skills from configured directories.
func (l *Loader) LoadAll() ([]*Skill, error) {
	var skills []*Skill

	for _, dir := range l.dirs {
		// Expand ~ to home directory
		if strings.HasPrefix(dir, "~") {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, dir[1:])
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Skip directories that don't exist
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillDir := filepath.Join(dir, entry.Name())
			skillFile := filepath.Join(skillDir, "SKILL.md")

			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				continue
			}

			skill, err := LoadSkill(skillFile)
			if err != nil {
				continue // Skip invalid skills
			}

			skills = append(skills, skill)
		}
	}

	return skills, nil
}

// LoadSkill loads a single skill from a SKILL.md file.
func LoadSkill(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	skill, err := ParseSkill(string(content))
	if err != nil {
		return nil, err
	}

	skill.Location = path
	return skill, nil
}

// ParseSkill parses a skill from markdown content with YAML frontmatter.
func ParseSkill(content string) (*Skill, error) {
	// Split frontmatter and body
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, nil // No valid frontmatter
	}

	frontmatter := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	// Parse frontmatter - metadata can be a string or a map
	var fm struct {
		Name        string      `yaml:"name"`
		Description string      `yaml:"description"`
		Homepage    string      `yaml:"homepage"`
		Metadata    interface{} `yaml:"metadata"` // Can be string or map
	}

	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil, err
	}

	skill := &Skill{
		Name:        fm.Name,
		Description: fm.Description,
		Homepage:    fm.Homepage,
		Content:     body,
	}

	// Parse metadata - handle both string and map formats
	if fm.Metadata != nil {
		skill.Metadata = parseMetadata(fm.Metadata)
	}

	return skill, nil
}

// parseMetadata handles metadata that can be either a JSON string or a YAML map
func parseMetadata(data interface{}) *ClawdbotMetadata {
	// If it's a string (quoted JSON in YAML), parse it as JSON
	if s, ok := data.(string); ok {
		var wrapper struct {
			Clawdbot *ClawdbotMetadata `json:"clawdbot"`
		}
		if err := json.Unmarshal([]byte(s), &wrapper); err == nil && wrapper.Clawdbot != nil {
			return wrapper.Clawdbot
		}
		return nil
	}

	// If it's a map (unquoted JSON parsed as YAML), extract clawdbot key
	if m, ok := data.(map[string]interface{}); ok {
		clawdbotData, exists := m["clawdbot"]
		if !exists {
			return nil
		}

		// Convert map to JSON then parse into struct
		jsonBytes, err := json.Marshal(clawdbotData)
		if err != nil {
			return nil
		}

		var meta ClawdbotMetadata
		if err := json.Unmarshal(jsonBytes, &meta); err != nil {
			return nil
		}
		return &meta
	}

	return nil
}

// IsEligible checks if a skill's requirements are met.
func (s *Skill) IsEligible() bool {
	if s.Metadata == nil || s.Metadata.Requires == nil {
		return true
	}

	// Check required binaries
	for _, bin := range s.Metadata.Requires.Bins {
		if !hasBinary(bin) {
			return false
		}
	}

	// Check required environment variables
	for _, env := range s.Metadata.Requires.Env {
		if os.Getenv(env) == "" {
			return false
		}
	}

	return true
}

// hasBinary checks if a binary is in PATH.
func hasBinary(name string) bool {
	_, err := findBinary(name)
	return err == nil
}

// findBinary finds a binary in PATH.
func findBinary(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, string(os.PathListSeparator))

	for _, dir := range paths {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", os.ErrNotExist
}

// FormatForPrompt formats skills for injection into the system prompt.
func FormatForPrompt(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")

	for _, skill := range skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString("    <name>")
		sb.WriteString(skill.Name)
		sb.WriteString("</name>\n")
		sb.WriteString("    <description>")
		sb.WriteString(skill.Description)
		sb.WriteString("</description>\n")
		sb.WriteString("    <location>")
		sb.WriteString(skill.Location)
		sb.WriteString("</location>\n")
		sb.WriteString("  </skill>\n")
	}

	sb.WriteString("</available_skills>")
	return sb.String()
}

// FilterEligible returns only skills whose requirements are met.
func FilterEligible(skills []*Skill) []*Skill {
	var eligible []*Skill
	for _, s := range skills {
		if s.IsEligible() {
			eligible = append(eligible, s)
		}
	}
	return eligible
}
