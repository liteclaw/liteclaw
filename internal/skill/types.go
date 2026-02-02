// Package skill provides skill loading and management for LiteClaw.
package skill

import (
	"encoding/json"
	"time"
)

// Skill represents a loaded skill from SKILL.md.
type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Homepage    string            `json:"homepage,omitempty"`
	Emoji       string            `json:"emoji,omitempty"`
	Author      string            `json:"author,omitempty"`
	Version     string            `json:"version,omitempty"`
	Metadata    *ClawdbotMetadata `json:"metadata,omitempty"`
	Source      SkillSource       `json:"source"`
	BaseDir     string            `json:"baseDir"`
	FilePath    string            `json:"filePath"`
	Content     string            `json:"-"` // Full SKILL.md content (not serialized)
}

// SkillSource indicates where the skill was loaded from.
type SkillSource string

const (
	SourceBundled   SkillSource = "bundled"
	SourceWorkspace SkillSource = "workspace"
	SourceManaged   SkillSource = "managed"
)

// ClawdbotMetadata represents the clawdbot-specific metadata in skills.
type ClawdbotMetadata struct {
	Emoji      string       `json:"emoji,omitempty"`
	SkillKey   string       `json:"skillKey,omitempty"`
	PrimaryEnv string       `json:"primaryEnv,omitempty"`
	Homepage   string       `json:"homepage,omitempty"`
	Always     bool         `json:"always,omitempty"`
	OS         []string     `json:"os,omitempty"`
	Requires   *Requires    `json:"requires,omitempty"`
	Install    []InstallOpt `json:"install,omitempty"`
}

// Requires specifies skill requirements.
type Requires struct {
	Bins    []string `json:"bins,omitempty"`
	AnyBins []string `json:"anyBins,omitempty"`
	Env     []string `json:"env,omitempty"`
	Config  []string `json:"config,omitempty"`
}

// InstallOpt describes an installation option for missing dependencies.
type InstallOpt struct {
	ID      string   `json:"id,omitempty"`
	Kind    string   `json:"kind"` // brew, node, go, uv, download
	Formula string   `json:"formula,omitempty"`
	Package string   `json:"package,omitempty"`
	Module  string   `json:"module,omitempty"`
	URL     string   `json:"url,omitempty"`
	Bins    []string `json:"bins,omitempty"`
	Label   string   `json:"label,omitempty"`
	OS      []string `json:"os,omitempty"`
}

// SkillStatus represents the eligibility status of a skill.
type SkillStatus struct {
	Skill          *Skill   `json:"skill"`
	Eligible       bool     `json:"eligible"`
	Disabled       bool     `json:"disabled"`
	MissingBins    []string `json:"missingBins,omitempty"`
	MissingEnv     []string `json:"missingEnv,omitempty"`
	UnsupportedOS  bool     `json:"unsupportedOS,omitempty"`
	InstallOptions []string `json:"installOptions,omitempty"`
}

// HubSkill represents a skill from ClawdHub registry.
type HubSkill struct {
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Author      string    `json:"author,omitempty"`
	Version     string    `json:"version"`
	Downloads   int       `json:"downloads,omitempty"`
	Stars       int       `json:"stars,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

// HubSearchResult represents search results from ClawdHub.
type HubSearchResult struct {
	Skills []HubSkillSummary `json:"skills"`
	Total  int               `json:"total"`
}

// HubSkillSummary represents a summary of a skill from search results.
type HubSkillSummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// LockFileEntry represents an entry in the skill lock file.
type LockFileEntry struct {
	Slug      string    `json:"slug"`
	Version   string    `json:"version"`
	Hash      string    `json:"hash"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LockFile represents the .liteclaw/skills.lock.json file.
type LockFile struct {
	Skills   map[string]LockFileEntry `json:"skills"`
	UpdateAt time.Time                `json:"updatedAt"`
}

// ParseMetadataJSON parses the metadata JSON from frontmatter.
func ParseMetadataJSON(raw string) (*ClawdbotMetadata, error) {
	if raw == "" {
		return nil, nil
	}

	var wrapper struct {
		Clawdbot *ClawdbotMetadata `json:"clawdbot"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Clawdbot, nil
}
