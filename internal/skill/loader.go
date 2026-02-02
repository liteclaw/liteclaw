package skill

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LoadFromDir loads all skills from a directory.
func LoadFromDir(dir string, source SkillSource) ([]*Skill, error) {
	var skills []*Skill

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return skills, nil
		}
		return nil, err
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

		skill, err := LoadSkillFile(skillFile, source, skillDir)
		if err != nil {
			// Skip invalid skills
			continue
		}

		skills = append(skills, skill)
	}

	return skills, nil
}

// LoadSkillFile loads a skill from a SKILL.md file.
func LoadSkillFile(filePath string, source SkillSource, baseDir string) (*Skill, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	skill := &Skill{
		Source:   source,
		BaseDir:  baseDir,
		FilePath: filePath,
		Content:  string(content),
	}

	// Parse frontmatter
	parseFrontmatter(skill, string(content))

	// If name is empty, use directory name
	if skill.Name == "" {
		skill.Name = filepath.Base(baseDir)
	}

	return skill, nil
}

// parseFrontmatter extracts YAML frontmatter from SKILL.md content.
func parseFrontmatter(skill *Skill, content string) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return
	}

	// Find closing ---
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx <= 0 {
		return
	}

	// Parse frontmatter lines
	for i := 1; i < endIdx; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = value
		case "homepage":
			skill.Homepage = value
		case "author":
			skill.Author = value
		case "version":
			skill.Version = value
		case "metadata":
			if meta, err := ParseMetadataJSON(value); err == nil && meta != nil {
				skill.Metadata = meta
				if meta.Emoji != "" {
					skill.Emoji = meta.Emoji
				}
				if meta.Homepage != "" && skill.Homepage == "" {
					skill.Homepage = meta.Homepage
				}
			}
		}
	}
}

// CheckEligibility checks if a skill is eligible to be used.
func CheckEligibility(skill *Skill) *SkillStatus {
	status := &SkillStatus{
		Skill:    skill,
		Eligible: true,
	}

	meta := skill.Metadata
	if meta == nil {
		return status
	}

	// Check OS requirements
	if len(meta.OS) > 0 {
		osMatches := false
		currentOS := runtime.GOOS
		for _, os := range meta.OS {
			if os == currentOS {
				osMatches = true
				break
			}
		}
		if !osMatches {
			status.Eligible = false
			status.UnsupportedOS = true
		}
	}

	// Check required binaries
	if meta.Requires != nil {
		for _, bin := range meta.Requires.Bins {
			if !hasBinary(bin) {
				status.MissingBins = append(status.MissingBins, bin)
				status.Eligible = false
			}
		}

		// Check anyBins (at least one required)
		if len(meta.Requires.AnyBins) > 0 {
			hasAny := false
			for _, bin := range meta.Requires.AnyBins {
				if hasBinary(bin) {
					hasAny = true
					break
				}
			}
			if !hasAny {
				status.MissingBins = append(status.MissingBins, "(any of: "+strings.Join(meta.Requires.AnyBins, ", ")+")")
				status.Eligible = false
			}
		}

		// Check required environment variables
		for _, env := range meta.Requires.Env {
			if os.Getenv(env) == "" {
				status.MissingEnv = append(status.MissingEnv, env)
				status.Eligible = false
			}
		}
	}

	// Collect install options
	for _, install := range meta.Install {
		if install.Label != "" {
			status.InstallOptions = append(status.InstallOptions, install.Label)
		}
	}

	return status
}

// hasBinary checks if a binary is available in PATH.
func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// LoadAllSkills loads skills from all configured directories.
func LoadAllSkills(bundledDir, managedDir, workspaceDir string) ([]*Skill, error) {
	skillMap := make(map[string]*Skill)

	// Load in order of precedence (later overwrites earlier)
	// 1. Bundled (lowest priority)
	if bundledDir != "" {
		bundled, err := LoadFromDir(bundledDir, SourceBundled)
		if err == nil {
			for _, s := range bundled {
				skillMap[s.Name] = s
			}
		}
	}

	// 2. Managed (~/.liteclaw/skills)
	if managedDir != "" {
		managed, err := LoadFromDir(managedDir, SourceManaged)
		if err == nil {
			for _, s := range managed {
				skillMap[s.Name] = s
			}
		}
	}

	// 3. Workspace (highest priority)
	if workspaceDir != "" {
		workspace, err := LoadFromDir(filepath.Join(workspaceDir, "skills"), SourceWorkspace)
		if err == nil {
			for _, s := range workspace {
				skillMap[s.Name] = s
			}
		}
	}

	// Convert map to slice
	var skills []*Skill
	for _, s := range skillMap {
		skills = append(skills, s)
	}

	return skills, nil
}

// BuildSkillsPrompt builds the skills prompt for the agent.
func BuildSkillsPrompt(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")

	for _, skill := range skills {
		emoji := skill.Emoji
		if emoji == "" {
			emoji = "🧩"
		}

		sb.WriteString("  <skill>\n")
		sb.WriteString("    <name>" + skill.Name + "</name>\n")
		sb.WriteString("    <emoji>" + emoji + "</emoji>\n")
		if skill.Description != "" {
			sb.WriteString("    <description>" + skill.Description + "</description>\n")
		}
		sb.WriteString("    <location>" + skill.FilePath + "</location>\n")
		sb.WriteString("  </skill>\n")
	}

	sb.WriteString("</available_skills>\n")
	return sb.String()
}

// GetSkillSummary returns a brief summary for a skill.
func GetSkillSummary(skill *Skill) string {
	var parts []string

	if skill.Emoji != "" {
		parts = append(parts, skill.Emoji)
	}

	parts = append(parts, skill.Name)

	if skill.Description != "" {
		desc := skill.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		parts = append(parts, "-", desc)
	}

	return strings.Join(parts, " ")
}

// ReadSkillContent reads the full content of a SKILL.md file.
func ReadSkillContent(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return sb.String(), nil
}
