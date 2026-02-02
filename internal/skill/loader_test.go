package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMetadataJSON(t *testing.T) {
	raw := `{"clawdbot": {"emoji": "🧪", "always": true}}`
	meta, err := ParseMetadataJSON(raw)
	assert.NoError(t, err)
	assert.NotNil(t, meta)
	assert.Equal(t, "🧪", meta.Emoji)
	assert.True(t, meta.Always)
}

func TestLoadSkillFile(t *testing.T) {
	tempDir := t.TempDir()
	skillPath := filepath.Join(tempDir, "SKILL.md")

	content := `---
name: test-skill
description: A test
metadata: {"clawdbot": {"emoji": "🧩"}}
---
# Skill Content`

	require.NoError(t, os.WriteFile(skillPath, []byte(content), 0644))

	s, err := LoadSkillFile(skillPath, SourceBundled, tempDir)
	assert.NoError(t, err)
	assert.Equal(t, "test-skill", s.Name)
	assert.Equal(t, "A test", s.Description)
	assert.Equal(t, "🧩", s.Emoji)
}

func TestCheckEligibility(t *testing.T) {
	s := &Skill{
		Name: "test",
		Metadata: &ClawdbotMetadata{
			Requires: &Requires{
				Bins: []string{"non-existent-binary-xyz-123"},
			},
		},
	}

	status := CheckEligibility(s)
	assert.False(t, status.Eligible)
	assert.Contains(t, status.MissingBins, "non-existent-binary-xyz-123")

	s2 := &Skill{
		Name: "test-ok",
		Metadata: &ClawdbotMetadata{
			Requires: &Requires{
				Bins: []string{"go"}, // Should exist on dev machine
			},
		},
	}
	status2 := CheckEligibility(s2)
	assert.True(t, status2.Eligible)
}
