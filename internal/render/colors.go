package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/jarrodwatts/claude-hud-go/internal/config"
)

const Reset = "\x1b[0m"

const (
	ansiDim           = "\x1b[2m"
	ansiRed           = "\x1b[31m"
	ansiGreen         = "\x1b[32m"
	ansiYellow        = "\x1b[33m"
	ansiMagenta       = "\x1b[35m"
	ansiCyan          = "\x1b[36m"
	ansiBrightBlue    = "\x1b[94m"
	ansiBrightMagenta = "\x1b[95m"
	ansiClaudeOrange  = "\x1b[38;5;208m"
)

var ansiByName = map[string]string{
	"dim": ansiDim, "red": ansiRed, "green": ansiGreen, "yellow": ansiYellow,
	"magenta": ansiMagenta, "cyan": ansiCyan, "brightBlue": ansiBrightBlue,
	"brightMagenta": ansiBrightMagenta,
}

func hexToAnsi(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex[1:3], "%02x", &r)
	fmt.Sscanf(hex[3:5], "%02x", &g)
	fmt.Sscanf(hex[5:7], "%02x", &b)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

// resolveAnsi accepts a named preset, a 256-color index (float64 from JSON), or
// a #rrggbb hex string.
func resolveAnsi(value any, fallback string) string {
	switch v := value.(type) {
	case nil:
		return fallback
	case float64:
		return fmt.Sprintf("\x1b[38;5;%dm", int(v))
	case int:
		return fmt.Sprintf("\x1b[38;5;%dm", v)
	case string:
		if strings.HasPrefix(v, "#") && len(v) == 7 {
			return hexToAnsi(v)
		}
		if a, ok := ansiByName[v]; ok {
			return a
		}
	}
	return fallback
}

func colorize(text, color string) string { return color + text + Reset }

func withOverride(text string, value any, fallback string) string {
	return colorize(text, resolveAnsi(value, fallback))
}

func green(t string) string        { return colorize(t, ansiGreen) }
func yellow(t string) string       { return colorize(t, ansiYellow) }
func red(t string) string          { return colorize(t, ansiRed) }
func cyan(t string) string         { return colorize(t, ansiCyan) }
func dim(t string) string          { return colorize(t, ansiDim) }
func claudeOrange(t string) string { return colorize(t, ansiClaudeOrange) }

func colModel(t string, c config.Colors) string     { return withOverride(t, c.Model, ansiCyan) }
func colProject(t string, c config.Colors) string   { return withOverride(t, c.Project, ansiYellow) }
func colGit(t string, c config.Colors) string       { return withOverride(t, c.Git, ansiMagenta) }
func colGitBranch(t string, c config.Colors) string { return withOverride(t, c.GitBranch, ansiCyan) }
func colLabel(t string, c config.Colors) string     { return withOverride(t, c.Label, ansiDim) }
func colCustom(t string, c config.Colors) string    { return withOverride(t, c.Custom, ansiClaudeOrange) }
func colWarning(t string, c config.Colors) string   { return colorize(t, resolveAnsi(c.Warning, ansiYellow)) }
func colCritical(t string, c config.Colors) string  { return colorize(t, resolveAnsi(c.Critical, ansiRed)) }

func getContextColor(percent float64, c config.Colors, warn, crit float64) string {
	if warn == 0 {
		warn = 70
	}
	if crit == 0 {
		crit = 85
	}
	if percent >= crit {
		return resolveAnsi(c.Critical, ansiRed)
	}
	if percent >= warn {
		return resolveAnsi(c.Warning, ansiYellow)
	}
	return resolveAnsi(c.Context, ansiGreen)
}

func getQuotaColor(percent float64, c config.Colors) string {
	if percent >= 90 {
		return resolveAnsi(c.Critical, ansiRed)
	}
	if percent >= 75 {
		return resolveAnsi(c.UsageWarning, ansiBrightMagenta)
	}
	return resolveAnsi(c.Usage, ansiBrightBlue)
}

func bar(percent float64, width int, color string, c config.Colors) string {
	if width < 0 {
		width = 0
	}
	p := math.Min(100, math.Max(0, percent))
	filled := int(math.Round(p / 100 * float64(width)))
	empty := width - filled
	fc := c.BarFilled
	if fc == "" {
		fc = "█"
	}
	ec := c.BarEmpty
	if ec == "" {
		ec = "░"
	}
	return color + strings.Repeat(fc, filled) + ansiDim + strings.Repeat(ec, empty) + Reset
}

func coloredBar(percent float64, width int, c config.Colors, warn, crit float64) string {
	return bar(percent, width, getContextColor(percent, c, warn, crit), c)
}

func quotaBar(percent float64, width int, c config.Colors) string {
	return bar(percent, width, getQuotaColor(percent, c), c)
}
