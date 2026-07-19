// Package render ports src/render/*: turning a resolved context into the HUD's
// multi-line ANSI output.
package render

import (
	"github.com/jarrodwatts/claude-hud-go/internal/config"
	"github.com/jarrodwatts/claude-hud-go/internal/configread"
	"github.com/jarrodwatts/claude-hud-go/internal/types"
)

// Context carries everything the renderers need (mirrors RenderContext).
type Context struct {
	Stdin           *types.StdinData
	Transcript      types.TranscriptData
	Counts          configread.Counts
	Git             *types.GitStatus
	Usage           *types.UsageData
	Memory          *types.MemoryInfo
	Config          config.HudConfig
	SessionDuration string
	OutputStyle     string
}
