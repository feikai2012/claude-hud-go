package render

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jarrodwatts/claude-hud-go/internal/stdinp"
)

// formatTokens mirrors utils/format.ts formatTokens.
func formatTokens(n int) string {
	f := float64(n)
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", f/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.0fk", f/1000)
	}
	return strconv.Itoa(n)
}

// formatContextValue mirrors utils/format.ts formatContextValue.
func (c *Context) formatContextValue(percent int, mode string) string {
	total := stdinp.TotalTokens(c.Stdin)
	size := 0
	if c.Config.Display.AutoCompactWindow != nil && *c.Config.Display.AutoCompactWindow > 0 {
		size = *c.Config.Display.AutoCompactWindow
	} else if c.Stdin != nil && c.Stdin.ContextWindow != nil {
		size = c.Stdin.ContextWindow.ContextWindowSize
	}
	switch mode {
	case "tokens":
		if size > 0 {
			return formatTokens(total) + "/" + formatTokens(size)
		}
		return formatTokens(total)
	case "both":
		if size > 0 {
			return fmt.Sprintf("%d%% (%s/%s)", percent, formatTokens(total), formatTokens(size))
		}
		return fmt.Sprintf("%d%%", percent)
	case "remaining":
		return fmt.Sprintf("%d%%", int(math.Max(0, float64(100-percent))))
	default:
		return fmt.Sprintf("%d%%", percent)
	}
}

// adaptiveBarWidth mirrors utils/terminal.ts getAdaptiveBarWidth.
func adaptiveBarWidth() int {
	cols := envColumns()
	if cols > 0 {
		switch {
		case cols >= 100:
			return 10
		case cols >= 60:
			return 6
		default:
			return 4
		}
	}
	return 10
}

func envColumns() int {
	if c := os.Getenv("COLUMNS"); c != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(c)); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// FormatSessionDuration mirrors index.ts formatSessionDuration.
func FormatSessionDuration(start *time.Time, now time.Time) string {
	if start == nil {
		return ""
	}
	mins := int(now.Sub(*start).Minutes())
	if mins < 1 {
		return "<1m"
	}
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dh %dm", mins/60, mins%60)
}

// formatResetTime renders a reset timestamp per the time format (relative/absolute).
func formatResetTime(reset *time.Time, mode string, now time.Time) string {
	if reset == nil {
		return ""
	}
	if mode == "absolute" {
		return "at " + reset.Format("15:04")
	}
	// relative
	d := reset.Sub(now)
	if d <= 0 {
		return "expired"
	}
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	h := mins / 60
	if h < 24 {
		if mins%60 == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, mins%60)
	}
	return fmt.Sprintf("%dd %dh", h/24, h%24)
}
