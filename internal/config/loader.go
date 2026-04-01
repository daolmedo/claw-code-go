package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds the merged configuration from all settings sources.
// Fields correspond to JSON keys in .claude/settings.json.
type Settings struct {
	Model          string   `json:"model,omitempty"`
	PermissionMode string   `json:"permissionMode,omitempty"`
	AllowedTools   []string `json:"allowedTools,omitempty"`
	BlockedTools   []string `json:"blockedTools,omitempty"`
	MaxTokens      int      `json:"maxTokens,omitempty"`
	Theme          string   `json:"theme,omitempty"`
}

// Load returns merged settings from (in order of increasing precedence):
//  1. ~/.claude/settings.json      (user global)
//  2. .claude/settings.json        (project)
//  3. .claude/settings.local.json  (local overrides, typically gitignored)
//
// CLI flag overrides are applied by the caller after Load returns.
func Load() *Settings {
	s := &Settings{}

	homeDir, _ := os.UserHomeDir()

	var sources []string
	if homeDir != "" {
		sources = append(sources, filepath.Join(homeDir, ".claude", "settings.json"))
	}
	sources = append(sources,
		filepath.Join(".claude", "settings.json"),
		filepath.Join(".claude", "settings.local.json"),
	)

	for _, path := range sources {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var patch Settings
		if err := json.Unmarshal(data, &patch); err != nil {
			continue
		}
		merge(s, &patch)
	}

	return s
}

// merge applies non-zero fields from src into dst.
func merge(dst, src *Settings) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.PermissionMode != "" {
		dst.PermissionMode = src.PermissionMode
	}
	if len(src.AllowedTools) > 0 {
		dst.AllowedTools = src.AllowedTools
	}
	if len(src.BlockedTools) > 0 {
		dst.BlockedTools = src.BlockedTools
	}
	if src.MaxTokens != 0 {
		dst.MaxTokens = src.MaxTokens
	}
	if src.Theme != "" {
		dst.Theme = src.Theme
	}
}

// WriteProject writes settings to .claude/settings.json, merging with any
// existing content to preserve unmanaged fields (e.g. mcpServers, rules).
func WriteProject(s *Settings) error {
	if err := os.MkdirAll(".claude", 0o755); err != nil {
		return err
	}

	// Read existing settings first to preserve unmanaged fields.
	existing := map[string]any{}
	if data, err := os.ReadFile(filepath.Join(".claude", "settings.json")); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Overlay our managed fields.
	if s.Model != "" {
		existing["model"] = s.Model
	}
	if s.PermissionMode != "" {
		existing["permissionMode"] = s.PermissionMode
	}
	if s.AllowedTools != nil {
		existing["allowedTools"] = s.AllowedTools
	}
	if s.BlockedTools != nil {
		existing["blockedTools"] = s.BlockedTools
	}
	if s.MaxTokens != 0 {
		existing["maxTokens"] = s.MaxTokens
	}
	if s.Theme != "" {
		existing["theme"] = s.Theme
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(".claude", "settings.json"), data, 0o644)
}

// InitProject creates .claude/settings.json with default values.
// Returns os.ErrExist if the file already exists.
func InitProject(model string) error {
	path := filepath.Join(".claude", "settings.json")
	if _, err := os.Stat(path); err == nil {
		return os.ErrExist
	}
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	defaults := &Settings{
		Model:          model,
		PermissionMode: "default",
		AllowedTools:   []string{},
		BlockedTools:   []string{},
	}
	return WriteProject(defaults)
}
