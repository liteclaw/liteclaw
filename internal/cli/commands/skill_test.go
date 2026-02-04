package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillListCommand(t *testing.T) {
	// Setup temp skills dir
	tempDir := t.TempDir()
	bundledDir := filepath.Join(tempDir, "bundled")
	managedDir := filepath.Join(tempDir, "managed")

	// Pre-create structure: bundled/test-skill/SKILL.md
	skillDir := filepath.Join(bundledDir, "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.MkdirAll(managedDir, 0755))

	// Create a mock skill
	skillContent := `---
name: test-skill
description: A test skill
---
# Test Skill`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644))

	// Set overrides
	_ = os.Setenv("LITECLAW_BUNDLED_SKILLS_DIR", bundledDir)
	_ = os.Setenv("LITECLAW_MANAGED_SKILLS_DIR", managedDir)
	defer func() { _ = os.Unsetenv("LITECLAW_BUNDLED_SKILLS_DIR") }()
	defer func() { _ = os.Unsetenv("LITECLAW_MANAGED_SKILLS_DIR") }()

	cmd := newSkillListCommand()

	out := CaptureStdout(func() {
		// Set showAll to true to see our mock skill
		cmd.SetArgs([]string{"--all"})
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	assert.Contains(t, out, "test-skill")
	assert.Contains(t, out, "A test skill")
}
