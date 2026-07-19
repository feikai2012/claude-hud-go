package config

import (
	"os"
	"path/filepath"
	"strings"
)

// pluginDirName is deliberately distinct from the TypeScript "claude-hud" so a
// Go install never collides with an existing TS install's cache/config.
const pluginDirName = "claude-hud-go"

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func expandHomePrefix(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return filepath.Join(home, p[2:])
	}
	return p
}

// GetClaudeConfigDir resolves CLAUDE_CONFIG_DIR (with ~ expansion), else ~/.claude.
func GetClaudeConfigDir() string {
	home := homeDir()
	env := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if env == "" {
		return filepath.Join(home, ".claude")
	}
	abs, err := filepath.Abs(expandHomePrefix(env, home))
	if err != nil {
		return expandHomePrefix(env, home)
	}
	return abs
}

// GetHudPluginDir returns <configDir>/plugins/claude-hud-go.
func GetHudPluginDir() string {
	return filepath.Join(GetClaudeConfigDir(), "plugins", pluginDirName)
}

// GetConfigPath returns the user config.json path.
func GetConfigPath() string {
	return filepath.Join(GetHudPluginDir(), "config.json")
}
