// Command claude-hud is the Go port of the claude-hud statusline plugin. It
// reads Claude Code's JSON from stdin, parses the session transcript and config
// files, and writes the rendered HUD to stdout. Mirrors src/index.ts main().
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/jarrodwatts/claude-hud-go/internal/config"
	"github.com/jarrodwatts/claude-hud-go/internal/configread"
	"github.com/jarrodwatts/claude-hud-go/internal/gitstat"
	"github.com/jarrodwatts/claude-hud-go/internal/i18n"
	"github.com/jarrodwatts/claude-hud-go/internal/render"
	"github.com/jarrodwatts/claude-hud-go/internal/stdinp"
	"github.com/jarrodwatts/claude-hud-go/internal/transcript"
)

// isHudDisabled mirrors index.ts isHudDisabled (CLAUDE_HUD_DISABLE).
func isHudDisabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("CLAUDE_HUD_DISABLE")))
	if v == "" {
		return false
	}
	return v != "0" && v != "false" && v != "off" && v != "no"
}

func main() {
	if isHudDisabled() {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[claude-hud] Error:", r)
		}
	}()

	stdin := stdinp.Read()
	debugLog(stdin != nil)

	cfg := config.Load()
	i18n.SetLanguage(cfg.Language)

	if stdin == nil {
		// No stdin (setup verification path).
		fmt.Println(i18n.T("init.initializing"))
		if isMacOS() {
			fmt.Println(i18n.T("init.macosNote"))
		}
		return
	}

	tr := transcript.Parse(stdin.TranscriptPath)
	counts := configread.Count(stdin.Cwd)

	ctx := &render.Context{
		Stdin:           stdin,
		Transcript:      tr,
		Counts:          counts,
		Config:          cfg,
		SessionDuration: render.FormatSessionDuration(tr.SessionStart, time.Now()),
		OutputStyle:     counts.OutputStyle,
	}
	if cfg.GitStatus.Enabled {
		ctx.Git = gitstat.Status(stdin.Cwd)
	}
	if cfg.Display.ShowUsage {
		ctx.Usage = stdinp.GetUsageFromStdin(stdin)
	}

	for _, line := range ctx.Render() {
		fmt.Println(line)
	}
}

func isMacOS() bool {
	return runtime.GOOS == "darwin"
}

// debugLog appends one line per invocation when CLAUDE_HUD_DEBUG_LOG points at
// a file. Used to verify Claude Code actually spawns the HUD.
func debugLog(gotStdin bool) {
	path := os.Getenv("CLAUDE_HUD_DEBUG_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s stdin=%v cols=%q\n", time.Now().Format(time.RFC3339), gotStdin, os.Getenv("COLUMNS"))
}
