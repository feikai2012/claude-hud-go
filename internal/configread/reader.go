// Package configread ports src/config-reader.ts: counting CLAUDE.md files,
// rules (*.md under rules/), MCP servers, hooks, and the active outputStyle.
//
// The TypeScript version adds a sentinel-based on-disk cache; here the counts
// are recomputed each invocation (cheap filesystem stats) to keep the port
// simple. The resulting counts match computeConfigCountsFresh().
package configread

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/jarrodwatts/claude-hud-go/internal/config"
)

// Counts is the resolved configuration inventory.
type Counts struct {
	ClaudeMd    int
	Rules       int
	Mcp         int
	Hooks       int
	OutputStyle string
}

// Count computes the configuration counts for the given cwd.
func Count(cwd string) Counts {
	claudeDir := config.GetClaudeConfigDir()
	claudeJSON := claudeDir + ".json"

	var c Counts
	userMcp := map[string]bool{}
	projectMcp := map[string]bool{}

	// --- user scope ---
	if fileExists(filepath.Join(claudeDir, "CLAUDE.md")) {
		c.ClaudeMd++
	}
	c.Rules += countRulesInDir(filepath.Join(claudeDir, "rules"), map[string]bool{})

	userSettings := filepath.Join(claudeDir, "settings.json")
	for name := range mcpServerNames(userSettings) {
		userMcp[name] = true
	}
	c.Hooks += countHooks(userSettings)
	if v := stringSetting(userSettings, "outputStyle"); v != "" {
		c.OutputStyle = v
	}
	if v := stringSetting(filepath.Join(claudeDir, "settings.local.json"), "outputStyle"); v != "" {
		c.OutputStyle = v
	}
	for name := range mcpServerNames(claudeJSON) {
		userMcp[name] = true
	}
	for name := range disabledMcp(claudeJSON, "disabledMcpServers") {
		delete(userMcp, name)
	}

	// --- project scope ---
	if cwd != "" {
		projectClaudeDir := filepath.Join(cwd, ".claude")
		overlaps := sameLocation(projectClaudeDir, claudeDir)

		if fileExists(filepath.Join(cwd, "CLAUDE.md")) {
			c.ClaudeMd++
		}
		if fileExists(filepath.Join(cwd, "CLAUDE.local.md")) {
			c.ClaudeMd++
		}
		if !overlaps && fileExists(filepath.Join(cwd, ".claude", "CLAUDE.md")) {
			c.ClaudeMd++
		}
		if fileExists(filepath.Join(cwd, ".claude", "CLAUDE.local.md")) {
			c.ClaudeMd++
		}
		if !overlaps {
			c.Rules += countRulesInDir(filepath.Join(cwd, ".claude", "rules"), map[string]bool{})
		}

		mcpJSON := mcpServerNames(filepath.Join(cwd, ".mcp.json"))
		projectSettings := filepath.Join(cwd, ".claude", "settings.json")
		if !overlaps {
			for name := range mcpServerNames(projectSettings) {
				projectMcp[name] = true
			}
			c.Hooks += countHooks(projectSettings)
			if v := stringSetting(projectSettings, "outputStyle"); v != "" {
				c.OutputStyle = v
			}
		}
		localSettings := filepath.Join(cwd, ".claude", "settings.local.json")
		for name := range mcpServerNames(localSettings) {
			projectMcp[name] = true
		}
		c.Hooks += countHooks(localSettings)
		if v := stringSetting(localSettings, "outputStyle"); v != "" {
			c.OutputStyle = v
		}
		for name := range disabledMcp(localSettings, "disabledMcpjsonServers") {
			delete(mcpJSON, name)
		}
		for name := range mcpJSON {
			projectMcp[name] = true
		}
	}

	c.Mcp = len(userMcp) + len(projectMcp)
	return c
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func readJSON(p string) map[string]interface{} {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return m
}

func mcpServerNames(p string) map[string]bool {
	out := map[string]bool{}
	m := readJSON(p)
	if m == nil {
		return out
	}
	if servers, ok := m["mcpServers"].(map[string]interface{}); ok {
		for k := range servers {
			out[k] = true
		}
	}
	return out
}

func disabledMcp(p, key string) map[string]bool {
	out := map[string]bool{}
	m := readJSON(p)
	if m == nil {
		return out
	}
	if arr, ok := m[key].([]interface{}); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				out[s] = true
			}
		}
	}
	return out
}

func countHooks(p string) int {
	m := readJSON(p)
	if m == nil {
		return 0
	}
	if hooks, ok := m["hooks"].(map[string]interface{}); ok {
		return len(hooks)
	}
	return 0
}

func stringSetting(p, key string) string {
	m := readJSON(p)
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func countRulesInDir(dir string, visited map[string]bool) int {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return 0
	}
	fi, err := os.Stat(real)
	if err != nil || !fi.IsDir() || visited[real] {
		return 0
	}
	visited[real] = true
	entries, err := os.ReadDir(real)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		full := filepath.Join(real, e.Name())
		if e.IsDir() {
			count += countRulesInDir(full, visited)
		} else if strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count
}

func sameLocation(a, b string) bool {
	na := normalizePath(a)
	nb := normalizePath(b)
	if na == nb {
		return true
	}
	ra, ea := filepath.EvalSymlinks(a)
	rb, eb := filepath.EvalSymlinks(b)
	if ea != nil || eb != nil {
		return false
	}
	return normalizePath(ra) == normalizePath(rb)
}

func normalizePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	abs = filepath.Clean(abs)
	if os.PathSeparator == '\\' {
		return strings.ToLower(abs)
	}
	return abs
}
