package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/liteclaw/liteclaw/internal/config"
	"github.com/liteclaw/liteclaw/internal/skill"
)

// DefaultManagedSkillsDir returns the default managed skills directory.
// Uses config.StateDir() (~/.liteclaw/skills).
func DefaultManagedSkillsDir() string {
	if override := os.Getenv("LITECLAW_MANAGED_SKILLS_DIR"); override != "" {
		return override
	}
	return filepath.Join(config.StateDir(), "skills")
}

// DefaultBundledSkillsDir returns the bundled skills directory.
func DefaultBundledSkillsDir() string {
	if override := os.Getenv("LITECLAW_BUNDLED_SKILLS_DIR"); override != "" {
		return override
	}
	// Get executable directory
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exeDir := filepath.Dir(exe)

	// Check for skills directory relative to executable
	candidates := []string{
		filepath.Join(exeDir, "skills"),
		filepath.Join(exeDir, "..", "skills"),
		filepath.Join(exeDir, "..", "..", "skills"),
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	// Fall back to current directory
	if info, err := os.Stat("skills"); err == nil && info.IsDir() {
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, "skills")
	}

	return ""
}

// NewSkillCommand creates the skill management command.
func NewSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage agent skills",
		Long: `Manage agent skills - list, install, remove, and search skills.

Skills are markdown files with YAML frontmatter that extend agent capabilities
by providing specialized knowledge and workflows.

Skills are loaded from:
  - ~/.liteclaw/skills/ (managed/installed skills)
  - ./skills/ (bundled skills)`,
		Example: `  # List all available skills
  liteclaw skill list

  # Search for weather skills on ClawdHub
  liteclaw skill search weather

  # Install a skill from ClawdHub
  liteclaw skill install solar-weather

  # Check skill eligibility and requirements
  liteclaw skill check

  # Get detailed info about a skill
  liteclaw skill info weather`,
	}

	// Add subcommands
	cmd.AddCommand(newSkillListCommand())
	cmd.AddCommand(newSkillInfoCommand())
	cmd.AddCommand(newSkillInstallCommand())
	cmd.AddCommand(newSkillRemoveCommand())
	cmd.AddCommand(newSkillSearchCommand())
	cmd.AddCommand(newSkillCheckCommand())
	cmd.AddCommand(newSkillSyncCommand())

	return cmd
}

// newSkillListCommand creates the 'skill list' subcommand.
func newSkillListCommand() *cobra.Command {
	var (
		showAll      bool
		showEligible bool
		jsonOutput   bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available skills",
		Long:    "List all available skills, their status, and eligibility.",
		Example: `  # List eligible skills only (default)
  liteclaw skill list

  # List all skills including ineligible ones
  liteclaw skill list --all

  # List only eligible skills
  liteclaw skill list --eligible

  # Output as JSON
  liteclaw skill list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillList(showAll, showEligible, jsonOutput)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all skills including ineligible ones")
	cmd.Flags().BoolVarP(&showEligible, "eligible", "e", false, "Show only eligible skills")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")

	return cmd
}

func runSkillList(showAll, showEligible, jsonOutput bool) error {
	bundledDir := DefaultBundledSkillsDir()
	managedDir := DefaultManagedSkillsDir()
	cwd, _ := os.Getwd()

	skills, err := skill.LoadAllSkills(bundledDir, managedDir, cwd)
	if err != nil {
		return fmt.Errorf("failed to load skills: %w", err)
	}

	if len(skills) == 0 {
		fmt.Println("No skills found.")
		fmt.Println("")
		fmt.Println("Tip: Use 'liteclaw skill install <name>' to install skills from ClawdHub.")
		fmt.Println("     Or 'liteclaw skill search <query>' to find skills.")
		return nil
	}

	// Sort skills by name
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	// Check eligibility and filter
	var statuses []*skill.SkillStatus
	for _, s := range skills {
		status := skill.CheckEligibility(s)

		if showEligible && !status.Eligible {
			continue
		}
		if !showAll && !status.Eligible {
			continue
		}

		statuses = append(statuses, status)
	}

	if jsonOutput {
		return printJSON(statuses)
	}

	// Print table
	fmt.Printf("Skills (%d total)\n", len(statuses))
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("%-4s %-20s %-8s %s\n", "✓", "NAME", "SOURCE", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 70))

	for _, status := range statuses {
		checkMark := "✗"
		if status.Eligible {
			checkMark = "✓"
		}

		emoji := status.Skill.Emoji
		if emoji == "" {
			emoji = "🧩"
		}

		name := status.Skill.Name
		if len(name) > 18 {
			name = name[:15] + "..."
		}

		desc := status.Skill.Description
		if len(desc) > 35 {
			desc = desc[:32] + "..."
		}

		source := string(status.Skill.Source)
		if len(source) > 10 {
			source = source[:7] + "..."
		}

		fmt.Printf("%s %s %-20s %-8s %s\n", checkMark, emoji, name, source, desc)

		// Show missing requirements if not eligible
		if !status.Eligible && showAll {
			if len(status.MissingBins) > 0 {
				fmt.Printf("    └─ Missing bins: %s\n", strings.Join(status.MissingBins, ", "))
			}
			if len(status.MissingEnv) > 0 {
				fmt.Printf("    └─ Missing env: %s\n", strings.Join(status.MissingEnv, ", "))
			}
			if status.UnsupportedOS {
				fmt.Printf("    └─ Unsupported OS\n")
			}
		}
	}

	fmt.Println(strings.Repeat("-", 70))

	eligibleCount := 0
	for _, s := range statuses {
		if s.Eligible {
			eligibleCount++
		}
	}
	fmt.Printf("Eligible: %d/%d\n", eligibleCount, len(statuses))
	fmt.Println("")
	fmt.Println("Tip: Use 'liteclaw skill install <name>' to install new skills.")

	return nil
}

// newSkillInfoCommand creates the 'skill info' subcommand.
func newSkillInfoCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "info <skill-name>",
		Short: "Show detailed information about a skill",
		Long:  "Show detailed information about a skill including requirements and eligibility.",
		Example: `  # Show info about a local skill
  liteclaw skill info weather

  # Show skill details as JSON
  liteclaw skill info slack --json

  # Check requirements for a specific skill
  liteclaw skill info github`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillInfo(args[0], jsonOutput)
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")

	return cmd
}

func runSkillInfo(name string, jsonOutput bool) error {
	bundledDir := DefaultBundledSkillsDir()
	managedDir := DefaultManagedSkillsDir()
	cwd, _ := os.Getwd()

	skills, err := skill.LoadAllSkills(bundledDir, managedDir, cwd)
	if err != nil {
		return fmt.Errorf("failed to load skills: %w", err)
	}

	// Find the skill
	var found *skill.Skill
	for _, s := range skills {
		if strings.EqualFold(s.Name, name) {
			found = s
			break
		}
	}

	if found == nil {
		fmt.Printf("Skill not found: %s\n", name)
		fmt.Println("")
		fmt.Println("Tip: Use 'liteclaw skill search <query>' to find skills on ClawdHub.")
		return nil
	}

	status := skill.CheckEligibility(found)

	if jsonOutput {
		return printJSON(status)
	}

	// Print skill info
	emoji := found.Emoji
	if emoji == "" {
		emoji = "🧩"
	}

	fmt.Printf("%s %s\n", emoji, found.Name)
	fmt.Println(strings.Repeat("=", 50))

	if found.Description != "" {
		fmt.Printf("Description: %s\n", found.Description)
	}
	if found.Homepage != "" {
		fmt.Printf("Homepage: %s\n", found.Homepage)
	}
	if found.Author != "" {
		fmt.Printf("Author: %s\n", found.Author)
	}
	if found.Version != "" {
		fmt.Printf("Version: %s\n", found.Version)
	}
	fmt.Printf("Source: %s\n", found.Source)
	fmt.Printf("Path: %s\n", found.FilePath)
	fmt.Println("")

	// Eligibility
	if status.Eligible {
		fmt.Println("Status: ✓ Eligible")
	} else {
		fmt.Println("Status: ✗ Not Eligible")
		if len(status.MissingBins) > 0 {
			fmt.Printf("  Missing binaries: %s\n", strings.Join(status.MissingBins, ", "))
		}
		if len(status.MissingEnv) > 0 {
			fmt.Printf("  Missing env vars: %s\n", strings.Join(status.MissingEnv, ", "))
		}
		if status.UnsupportedOS {
			fmt.Println("  Unsupported OS")
		}
	}

	// Requirements
	if found.Metadata != nil && found.Metadata.Requires != nil {
		fmt.Println("")
		fmt.Println("Requirements:")
		if len(found.Metadata.Requires.Bins) > 0 {
			fmt.Printf("  Binaries: %s\n", strings.Join(found.Metadata.Requires.Bins, ", "))
		}
		if len(found.Metadata.Requires.Env) > 0 {
			fmt.Printf("  Environment: %s\n", strings.Join(found.Metadata.Requires.Env, ", "))
		}
		if len(found.Metadata.OS) > 0 {
			fmt.Printf("  OS: %s\n", strings.Join(found.Metadata.OS, ", "))
		}
	}

	// Install options
	if len(status.InstallOptions) > 0 {
		fmt.Println("")
		fmt.Println("Install Options:")
		for _, opt := range status.InstallOptions {
			fmt.Printf("  • %s\n", opt)
		}
	}

	return nil
}

// newSkillInstallCommand creates the 'skill install' subcommand.
func newSkillInstallCommand() *cobra.Command {
	var (
		version  string
		registry string
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "install <skill-slug>",
		Short: "Install a skill from ClawdHub",
		Long:  `Install a skill from the ClawdHub registry.`,
		Example: `  liteclaw skill install weather
  liteclaw skill install pdf --version 1.2.0
  liteclaw skill install my-skill --force --registry https://hub.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillInstall(args[0], version, registry, force)
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Install a specific version (default: latest)")
	cmd.Flags().StringVar(&registry, "registry", "", "ClawdHub registry URL")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing skill")

	return cmd
}

func runSkillInstall(slug, version, registry string, force bool) error {
	managedDir := DefaultManagedSkillsDir()
	bundledDir := DefaultBundledSkillsDir()

	// Check if already installed in managed directory
	skillDir := filepath.Join(managedDir, slug)
	if _, err := os.Stat(skillDir); err == nil && !force {
		fmt.Printf("Skill '%s' is already installed.\n", slug)
		fmt.Println("Use --force to overwrite.")
		return nil
	}

	// Ensure managed directory exists
	if err := os.MkdirAll(managedDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	// First, try to find in bundled skills
	if bundledDir != "" {
		bundledSkillDir := filepath.Join(bundledDir, slug)
		if info, err := os.Stat(bundledSkillDir); err == nil && info.IsDir() {
			fmt.Printf("Installing skill '%s' from bundled skills...\n", slug)
			if err := copySkillDir(bundledSkillDir, skillDir); err != nil {
				return fmt.Errorf("failed to copy skill: %w", err)
			}

			// Update lock file
			updateLockFileEntry(managedDir, slug, "bundled")

			fmt.Printf("✓ Successfully installed %s to %s\n", slug, skillDir)
			fmt.Println("")
			fmt.Println("The skill will be available in the next agent session.")
			return nil
		}
	}

	// Try ClawdHub registry
	hub := skill.NewHubClient(registry)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Printf("Fetching skill '%s' from ClawdHub...\n", slug)

	// Get skill info from ClawdHub
	info, err := hub.GetSkillInfo(ctx, slug)
	if err != nil {
		// ClawdHub unavailable or skill not found
		fmt.Printf("Could not fetch from ClawdHub: %v\n", err)
		fmt.Println("")
		fmt.Println("Available options:")
		fmt.Println("  1. Check if the skill exists in bundled skills with: liteclaw skill list --all")
		fmt.Println("  2. Browse ClawHub at: https://clawhub.ai")
		fmt.Println("  3. Install via npm: npm i -g clawdhub && clawdhub install " + slug)
		return nil
	}

	targetVersion := version
	if targetVersion == "" {
		targetVersion = info.Version
	}

	fmt.Printf("Installing %s@%s...\n", slug, targetVersion)

	// Download and extract
	if err := hub.Download(ctx, slug, targetVersion, managedDir); err != nil {
		return fmt.Errorf("failed to download skill: %w", err)
	}

	// Update lock file
	updateLockFileEntry(managedDir, slug, targetVersion)

	fmt.Printf("✓ Successfully installed %s@%s to %s\n", slug, targetVersion, skillDir)
	fmt.Println("")
	fmt.Println("The skill will be available in the next agent session.")

	return nil
}

// copySkillDir copies a skill directory to the target location.
func copySkillDir(src, dst string) error {
	// Remove existing directory
	_ = os.RemoveAll(dst)

	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, data, info.Mode())
	})
}

// updateLockFileEntry updates the lock file with a new skill entry.
func updateLockFileEntry(managedDir, slug, version string) {
	lockFile, err := skill.LoadLockFile(managedDir)
	if err != nil {
		lockFile = &skill.LockFile{Skills: make(map[string]skill.LockFileEntry)}
	}

	lockFile.Skills[slug] = skill.LockFileEntry{
		Slug:      slug,
		Version:   version,
		UpdatedAt: time.Now(),
	}

	if err := skill.SaveLockFile(managedDir, lockFile); err != nil {
		fmt.Printf("Warning: failed to update lock file: %v\n", err)
	}
}

// newSkillRemoveCommand creates the 'skill remove' subcommand.
func newSkillRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <skill-name>",
		Aliases: []string{"rm", "uninstall"},
		Short:   "Remove an installed skill",
		Long:    "Remove an installed skill from the managed skills directory.",
		Example: `  # Remove a skill
  liteclaw skill remove solar-weather

  # Uninstall using alias
  liteclaw skill rm slack-personal

  # Alternative alias
  liteclaw skill uninstall trello`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillRemove(args[0])
		},
	}

	return cmd
}

func runSkillRemove(name string) error {
	managedDir := DefaultManagedSkillsDir()

	if err := skill.RemoveSkill(managedDir, name); err != nil {
		return err
	}

	fmt.Printf("✓ Successfully removed skill '%s'\n", name)
	return nil
}

// newSkillSearchCommand creates the 'skill search' subcommand.
func newSkillSearchCommand() *cobra.Command {
	var (
		limit    int
		registry string
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for skills on ClawdHub",
		Long:  `Search for skills on the ClawdHub registry.`,
		Example: `  liteclaw skill search weather
  liteclaw skill search "image generation"
  liteclaw skill search pdf --limit 10
  liteclaw skill search weather --registry https://hub.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillSearch(args[0], limit, registry)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results")
	cmd.Flags().StringVar(&registry, "registry", "", "ClawdHub registry URL")

	return cmd
}

func runSkillSearch(query string, limit int, registry string) error {
	bundledDir := DefaultBundledSkillsDir()
	managedDir := DefaultManagedSkillsDir()
	cwd, _ := os.Getwd()

	query = strings.ToLower(query)
	fmt.Printf("Searching for '%s'...\n", query)
	fmt.Println("")

	// First, search local bundled skills
	localSkills, _ := skill.LoadAllSkills(bundledDir, managedDir, cwd)
	var localMatches []*skill.Skill
	for _, s := range localSkills {
		// Match by name or description
		nameLower := strings.ToLower(s.Name)
		descLower := strings.ToLower(s.Description)
		if strings.Contains(nameLower, query) || strings.Contains(descLower, query) {
			localMatches = append(localMatches, s)
		}
	}

	// Then try ClawdHub for remote skills
	hub := skill.NewHubClient(registry)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	remoteResult, remoteErr := hub.Search(ctx, query, limit)

	// Display local results
	if len(localMatches) > 0 {
		fmt.Printf("📦 Local Skills (%d match):\n", len(localMatches))
		fmt.Println(strings.Repeat("-", 70))
		fmt.Printf("%-20s %-10s %s\n", "NAME", "SOURCE", "DESCRIPTION")
		fmt.Println(strings.Repeat("-", 70))

		for _, s := range localMatches {
			name := s.Name
			if len(name) > 18 {
				name = name[:15] + "..."
			}
			desc := s.Description
			if len(desc) > 35 {
				desc = desc[:32] + "..."
			}
			source := string(s.Source)
			if len(source) > 10 {
				source = source[:7] + "..."
			}
			fmt.Printf("%-20s %-10s %s\n", name, source, desc)
		}
		fmt.Println("")
	}

	// Display remote results
	if remoteErr == nil && len(remoteResult.Skills) > 0 {
		fmt.Printf("☁️  ClawdHub Skills (%d match):\n", len(remoteResult.Skills))
		fmt.Println(strings.Repeat("-", 70))
		fmt.Printf("%-20s %-10s %s\n", "SLUG", "VERSION", "DESCRIPTION")
		fmt.Println(strings.Repeat("-", 70))

		for _, s := range remoteResult.Skills {
			slug := s.Slug
			if len(slug) > 18 {
				slug = slug[:15] + "..."
			}
			desc := s.Description
			if len(desc) > 35 {
				desc = desc[:32] + "..."
			}
			fmt.Printf("%-20s %-10s %s\n", slug, s.Version, desc)
		}
		fmt.Println("")
	} else if remoteErr != nil {
		// ClawdHub not available - show helpful message
		fmt.Println("☁️  ClawdHub: Not available (offline or not configured)")
		fmt.Println("")
	}

	// No results at all
	if len(localMatches) == 0 && (remoteErr != nil || len(remoteResult.Skills) == 0) {
		fmt.Println("No skills found matching your query.")
		fmt.Println("")
		fmt.Println("Tips:")
		fmt.Println("  • Try a different search term")
		fmt.Println("  • List all local skills: liteclaw skill list --all")
		fmt.Println("  • Use npx clawdhub to search the online registry")
		return nil
	}

	// Show install hint
	fmt.Println("Install with:")
	if len(localMatches) > 0 {
		fmt.Println("  liteclaw skill install <name>   (local bundled skill)")
	}
	if remoteErr == nil && len(remoteResult.Skills) > 0 {
		fmt.Println("  liteclaw skill install <slug>   (from ClawdHub)")
	}

	return nil
}

// newSkillCheckCommand creates the 'skill check' subcommand.
func newSkillCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [skill-name]",
		Short: "Check skill eligibility and requirements",
		Long:  "Check which skills are ready to use and which are missing requirements.",
		Example: `  # Check all skills
  liteclaw skill check

  # Check a specific skill (optional)
  liteclaw skill check weather`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runSkillCheck(name)
		},
	}

	return cmd
}

func runSkillCheck(name string) error {
	bundledDir := DefaultBundledSkillsDir()
	managedDir := DefaultManagedSkillsDir()
	cwd, _ := os.Getwd()

	skills, err := skill.LoadAllSkills(bundledDir, managedDir, cwd)
	if err != nil {
		return fmt.Errorf("failed to load skills: %w", err)
	}

	// Filter by name if provided
	if name != "" {
		var filtered []*skill.Skill
		for _, s := range skills {
			if strings.EqualFold(s.Name, name) {
				filtered = append(filtered, s)
				break
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("skill not found: %s", name)
		}
		skills = filtered
	}

	eligible := 0
	notEligible := 0

	var issues []string

	for _, s := range skills {
		status := skill.CheckEligibility(s)
		if status.Eligible {
			eligible++
		} else {
			notEligible++

			var missing []string
			if len(status.MissingBins) > 0 {
				missing = append(missing, fmt.Sprintf("bins: %s", strings.Join(status.MissingBins, ", ")))
			}
			if len(status.MissingEnv) > 0 {
				missing = append(missing, fmt.Sprintf("env: %s", strings.Join(status.MissingEnv, ", ")))
			}
			if status.UnsupportedOS {
				missing = append(missing, "unsupported OS")
			}

			issues = append(issues, fmt.Sprintf("  %s: %s", s.Name, strings.Join(missing, "; ")))
		}
	}

	if name != "" {
		fmt.Printf("Skill Check: %s\n", name)
	} else {
		fmt.Println("Skills Check")
	}
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Total: %d\n", len(skills))
	fmt.Printf("Eligible: %d ✓\n", eligible)
	fmt.Printf("Not Eligible: %d ✗\n", notEligible)

	if len(issues) > 0 {
		fmt.Println("")
		fmt.Println("Issues:")
		for _, issue := range issues {
			fmt.Println(issue)
		}
	}

	fmt.Println("")
	fmt.Println("Tip: Use 'liteclaw skill info <name>' for details on a specific skill.")

	return nil
}

// newSkillSyncCommand creates the 'skill sync' subcommand.
func newSkillSyncCommand() *cobra.Command {
	var (
		dryRun bool
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync bundled skills to managed directory",
		Long: `Sync all bundled skills to the managed skills directory (~/.liteclaw/skills/).

This copies skills from the bundled directory to the managed directory,
making them available for easy removal or updates.`,
		Example: `  # Sync all bundled skills
  liteclaw skill sync

  # Preview what would be synced without making changes
  liteclaw skill sync --dry-run

  # Force overwrite existing skills
  liteclaw skill sync --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillSync(dryRun, force)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be synced without making changes")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing skills in managed directory")

	return cmd
}

func runSkillSync(dryRun, force bool) error {
	bundledDir := DefaultBundledSkillsDir()
	managedDir := DefaultManagedSkillsDir()

	if bundledDir == "" {
		fmt.Println("No bundled skills directory found.")
		return nil
	}

	// Load bundled skills
	bundled, err := skill.LoadFromDir(bundledDir, skill.SourceBundled)
	if err != nil {
		return fmt.Errorf("failed to load bundled skills: %w", err)
	}

	if len(bundled) == 0 {
		fmt.Println("No bundled skills found.")
		return nil
	}

	// Ensure managed directory exists
	if !dryRun {
		if err := os.MkdirAll(managedDir, 0755); err != nil {
			return fmt.Errorf("failed to create managed directory: %w", err)
		}
	}

	synced := 0
	skipped := 0

	fmt.Printf("Syncing %d bundled skills to %s\n", len(bundled), managedDir)
	fmt.Println(strings.Repeat("-", 60))

	for _, s := range bundled {
		skillDir := filepath.Join(managedDir, s.Name)

		// Check if already exists
		if _, err := os.Stat(skillDir); err == nil {
			if !force {
				fmt.Printf("  ⏭ %s (already exists, use --force to overwrite)\n", s.Name)
				skipped++
				continue
			}
		}

		if dryRun {
			fmt.Printf("  📋 %s (would sync)\n", s.Name)
			synced++
		} else {
			if err := copySkillDir(s.BaseDir, skillDir); err != nil {
				fmt.Printf("  ✗ %s (failed: %v)\n", s.Name, err)
				continue
			}
			updateLockFileEntry(managedDir, s.Name, "bundled")
			fmt.Printf("  ✓ %s\n", s.Name)
			synced++
		}
	}

	fmt.Println(strings.Repeat("-", 60))
	if dryRun {
		fmt.Printf("Would sync: %d, Skipped: %d\n", synced, skipped)
	} else {
		fmt.Printf("Synced: %d, Skipped: %d\n", synced, skipped)
	}

	return nil
}

// printJSON prints data as JSON.
func printJSON(data interface{}) error {
	enc := os.Stdout
	encoder := json.NewEncoder(enc)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
