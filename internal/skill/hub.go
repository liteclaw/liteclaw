package skill

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultHubRegistry is the default ClawdHub registry URL.
	DefaultHubRegistry = "https://clawhub.ai"

	// HubTimeout is the default timeout for hub API requests.
	HubTimeout = 30 * time.Second
)

// HubClient provides methods to interact with ClawdHub registry.
type HubClient struct {
	Registry   string
	HTTPClient *http.Client
}

// NewHubClient creates a new ClawdHub client.
func NewHubClient(registry string) *HubClient {
	if registry == "" {
		registry = DefaultHubRegistry
	}
	return &HubClient{
		Registry: strings.TrimSuffix(registry, "/"),
		HTTPClient: &http.Client{
			Timeout: HubTimeout,
		},
	}
}

// Search searches for skills on ClawdHub using v1 API.
func (h *HubClient) Search(ctx context.Context, query string, limit int) (*HubSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	// Use /api/v1/search endpoint
	url := fmt.Sprintf("%s/api/v1/search?q=%s&limit=%d", h.Registry, query, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LiteClaw/1.0")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hub returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse v1 API response format: {"results": [...]}
	var apiResp struct {
		Results []struct {
			Score       float64 `json:"score"`
			Slug        string  `json:"slug"`
			DisplayName string  `json:"displayName"`
			Summary     string  `json:"summary"`
			Version     string  `json:"version"`
			UpdatedAt   int64   `json:"updatedAt"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to HubSearchResult
	result := &HubSearchResult{}
	for _, r := range apiResp.Results {
		result.Skills = append(result.Skills, HubSkillSummary{
			Slug:        r.Slug,
			Name:        r.DisplayName,
			Description: r.Summary,
			Version:     r.Version,
		})
	}

	return result, nil
}

// GetSkillInfo gets detailed information about a skill using v1 API.
func (h *HubClient) GetSkillInfo(ctx context.Context, slug string) (*HubSkill, error) {
	// Use /api/v1/skills/{slug} endpoint
	url := fmt.Sprintf("%s/api/v1/skills/%s", h.Registry, slug)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LiteClaw/1.0")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get skill info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("skill not found: %s", slug)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hub returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse v1 API response format
	var apiResp struct {
		Skill struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"displayName"`
			Summary     string `json:"summary"`
		} `json:"skill"`
		LatestVersion struct {
			Version string `json:"version"`
		} `json:"latestVersion"`
		Owner struct {
			Handle string `json:"handle"`
		} `json:"owner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &HubSkill{
		Slug:        apiResp.Skill.Slug,
		Name:        apiResp.Skill.DisplayName,
		Description: apiResp.Skill.Summary,
		Version:     apiResp.LatestVersion.Version,
		Author:      apiResp.Owner.Handle,
	}, nil
}

// Download downloads a skill from ClawdHub using v1 API.
func (h *HubClient) Download(ctx context.Context, slug string, version string, targetDir string) error {
	if version == "" {
		version = "latest"
	}

	// Use /api/v1/download endpoint
	url := fmt.Sprintf("%s/api/v1/download?slug=%s&version=%s", h.Registry, slug, version)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LiteClaw/1.0")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("skill or version not found: %s@%s", slug, version)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hub returned %d: %s", resp.StatusCode, string(body))
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "liteclaw-skill-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to download skill: %w", err)
	}

	// Extract to target directory
	skillDir := filepath.Join(targetDir, slug)
	if err := extractZip(tmpPath, skillDir); err != nil {
		return fmt.Errorf("failed to extract skill: %w", err)
	}

	return nil
}

// extractZip extracts a zip file to a directory.
func extractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Remove existing directory
	os.RemoveAll(destDir)

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		// Sanitize file path
		name := filepath.Clean(file.Name)
		if strings.HasPrefix(name, "..") {
			continue // Skip files that would escape destDir
		}

		// Handle root-level directory in zip (strip first component if it's a directory)
		parts := strings.Split(name, string(filepath.Separator))
		if len(parts) > 1 {
			// Skip the root directory name from the zip
			name = filepath.Join(parts[1:]...)
		} else if file.FileInfo().IsDir() {
			continue // Skip the root directory itself
		}

		if name == "" || name == "." {
			continue
		}

		destPath := filepath.Join(destDir, name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Extract file
		rc, err := file.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// ListInstalled lists installed skills from the managed directory.
func ListInstalled(managedDir string) ([]LockFileEntry, error) {
	lockPath := filepath.Join(managedDir, "..", "skills.lock.json")

	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var lockFile LockFile
	if err := json.Unmarshal(data, &lockFile); err != nil {
		return nil, err
	}

	var entries []LockFileEntry
	for _, entry := range lockFile.Skills {
		entries = append(entries, entry)
	}

	return entries, nil
}

// SaveLockFile saves the lock file after an install/update.
func SaveLockFile(managedDir string, lockFile *LockFile) error {
	lockPath := filepath.Join(managedDir, "..", "skills.lock.json")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return err
	}

	lockFile.UpdateAt = time.Now()

	data, err := json.MarshalIndent(lockFile, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(lockPath, data, 0644)
}

// LoadLockFile loads the lock file.
func LoadLockFile(managedDir string) (*LockFile, error) {
	lockPath := filepath.Join(managedDir, "..", "skills.lock.json")

	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &LockFile{Skills: make(map[string]LockFileEntry)}, nil
		}
		return nil, err
	}

	var lockFile LockFile
	if err := json.Unmarshal(data, &lockFile); err != nil {
		return nil, err
	}

	if lockFile.Skills == nil {
		lockFile.Skills = make(map[string]LockFileEntry)
	}

	return &lockFile, nil
}

// RemoveSkill removes an installed skill.
func RemoveSkill(managedDir, slug string) error {
	skillDir := filepath.Join(managedDir, slug)

	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return errors.New("skill not installed: " + slug)
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("failed to remove skill: %w", err)
	}

	// Update lock file
	lockFile, err := LoadLockFile(managedDir)
	if err != nil {
		return err
	}

	delete(lockFile.Skills, slug)
	return SaveLockFile(managedDir, lockFile)
}
