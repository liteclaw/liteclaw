package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkill(t *testing.T) {
	content := `---
name: test-skill
description: A test skill
homepage: https://example.com
---

# Test Skill

This is the skill body.
`

	skill, err := ParseSkill(content)
	if err != nil {
		t.Fatalf("Failed to parse skill: %v", err)
	}

	if skill == nil {
		t.Fatal("Expected skill, got nil")
	}

	if skill.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got %q", skill.Name)
	}

	if skill.Description != "A test skill" {
		t.Errorf("Expected description 'A test skill', got %q", skill.Description)
	}

	if skill.Homepage != "https://example.com" {
		t.Errorf("Expected homepage 'https://example.com', got %q", skill.Homepage)
	}

	if skill.Content == "" {
		t.Error("Expected non-empty content")
	}
}

func TestParseSkillWithMetadata(t *testing.T) {
	content := `---
name: docker
description: Docker container management
homepage: https://docker.com
metadata: '{"clawdbot":{"emoji":"🐳","requires":{"bins":["docker"]}}}'
---

# Docker

Manage containers.
`

	skill, err := ParseSkill(content)
	if err != nil {
		t.Fatalf("Failed to parse skill: %v", err)
	}

	if skill.Name != "docker" {
		t.Errorf("Expected name 'docker', got %q", skill.Name)
	}
}

func TestLoadSkill(t *testing.T) {
	// Create temp directory
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	_ = os.MkdirAll(skillDir, 0755)

	// Write SKILL.md
	skillPath := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: test-skill
description: Test description
---

# Test Skill
`
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// Load skill
	skill, err := LoadSkill(skillPath)
	if err != nil {
		t.Fatalf("Failed to load skill: %v", err)
	}

	if skill.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got %q", skill.Name)
	}

	if skill.Location != skillPath {
		t.Errorf("Expected location %q, got %q", skillPath, skill.Location)
	}
}

func TestLoaderLoadAll(t *testing.T) {
	// Create temp directory with skills
	dir := t.TempDir()

	// Create skill 1
	skill1Dir := filepath.Join(dir, "skill1")
	_ = os.MkdirAll(skill1Dir, 0755)
	_ = os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(`---
name: skill1
description: First skill
---
# Skill 1
`), 0644)

	// Create skill 2
	skill2Dir := filepath.Join(dir, "skill2")
	_ = os.MkdirAll(skill2Dir, 0755)
	_ = os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(`---
name: skill2
description: Second skill
---
# Skill 2
`), 0644)

	// Create directory without SKILL.md (should be skipped)
	_ = os.MkdirAll(filepath.Join(dir, "not-a-skill"), 0755)

	// Load all
	loader := NewLoader(dir)
	skills, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("Failed to load skills: %v", err)
	}

	if len(skills) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(skills))
	}
}

func TestFormatForPrompt(t *testing.T) {
	skills := []*Skill{
		{Name: "skill1", Description: "First skill", Location: "/path/to/skill1/SKILL.md"},
		{Name: "skill2", Description: "Second skill", Location: "/path/to/skill2/SKILL.md"},
	}

	output := FormatForPrompt(skills)

	if output == "" {
		t.Error("Expected non-empty output")
	}

	if !contains(output, "<available_skills>") {
		t.Error("Expected <available_skills> tag")
	}

	if !contains(output, "skill1") {
		t.Error("Expected skill1 in output")
	}

	if !contains(output, "skill2") {
		t.Error("Expected skill2 in output")
	}
}

func TestFilterEligible(t *testing.T) {
	skills := []*Skill{
		{Name: "eligible", Description: "No requirements"},
		{
			Name:        "requires-bin",
			Description: "Requires nonexistent binary",
			Metadata: &ClawdbotMetadata{
				Requires: &SkillRequires{
					Bins: []string{"nonexistent-binary-12345"},
				},
			},
		},
	}

	eligible := FilterEligible(skills)

	if len(eligible) != 1 {
		t.Errorf("Expected 1 eligible skill, got %d", len(eligible))
	}

	if eligible[0].Name != "eligible" {
		t.Errorf("Expected 'eligible' skill, got %q", eligible[0].Name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
