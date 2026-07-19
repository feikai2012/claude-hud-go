// Package config ports src/config.ts: the HUD configuration schema, defaults,
// and load/merge/validate logic.
//
// Merge strategy: Go's encoding/json unmarshals into a pre-populated struct,
// overwriting only the fields present in the user JSON and leaving the rest at
// their default values (recursively for nested structs). That gives the same
// "user overrides on top of defaults" behavior as mergeConfig() in the TS
// source. A normalize() pass afterward clamps/validates ranges the way the TS
// validators do.
package config

import (
	"encoding/json"
	"math"
	"os"
)

// AutocompactBufferPercent mirrors constants.ts.
const AutocompactBufferPercent = 0.165

// GitConfig ports the gitStatus block.
type GitConfig struct {
	Enabled              bool   `json:"enabled"`
	ShowDirty            bool   `json:"showDirty"`
	ShowAheadBehind      bool   `json:"showAheadBehind"`
	ShowFileStats        bool   `json:"showFileStats"`
	BranchOverflow       string `json:"branchOverflow"` // truncate | wrap
	PushWarningThreshold int    `json:"pushWarningThreshold"`
	PushCriticalThreshold int   `json:"pushCriticalThreshold"`
}

// Display ports the display block (the bulk of the ~50 options).
type Display struct {
	ShowModel             bool     `json:"showModel"`
	ShowProject           bool     `json:"showProject"`
	ShowAddedDirs         bool     `json:"showAddedDirs"`
	AddedDirsLayout       string   `json:"addedDirsLayout"` // inline | line
	ShowContextBar        bool     `json:"showContextBar"`
	ContextValue          string   `json:"contextValue"` // percent|tokens|remaining|both
	ShowConfigCounts      bool     `json:"showConfigCounts"`
	ShowCost              bool     `json:"showCost"`
	ShowRoutedCost        bool     `json:"showRoutedCost"`
	ShowDuration          bool     `json:"showDuration"`
	ShowSpeed             bool     `json:"showSpeed"`
	ShowTokenBreakdown    bool     `json:"showTokenBreakdown"`
	ShowUsage             bool     `json:"showUsage"`
	UsageValue            string   `json:"usageValue"` // percent | remaining
	UsageBarEnabled       bool     `json:"usageBarEnabled"`
	// ShowScopedUsage renders model-scoped weekly windows (e.g. the Fable
	// quota from rate_limits.model_scoped) on the usage line. Default true.
	ShowScopedUsage       bool     `json:"showScopedUsage"`
	ShowResetLabel        bool     `json:"showResetLabel"`
	UsageCompact          bool     `json:"usageCompact"`
	ShowTools             bool     `json:"showTools"`
	ShowSkills            bool     `json:"showSkills"`
	ShowMcp               bool     `json:"showMcp"`
	ToolNameMaxLength     int      `json:"toolNameMaxLength"`
	ToolsMaxVisible       int      `json:"toolsMaxVisible"`
	ShowAgents            bool     `json:"showAgents"`
	ShowTodos             bool     `json:"showTodos"`
	ShowSessionName       bool     `json:"showSessionName"`
	ShowAuth              bool     `json:"showAuth"`
	ShowAuthUser          bool     `json:"showAuthUser"`
	AuthUserLength        int      `json:"authUserLength"`
	ShowClaudeCodeVersion bool     `json:"showClaudeCodeVersion"`
	ShowEffortLevel       bool     `json:"showEffortLevel"`
	ShowMemoryUsage       bool     `json:"showMemoryUsage"`
	ShowPromptCache       bool     `json:"showPromptCache"`
	PromptCacheTtlSeconds int      `json:"promptCacheTtlSeconds"`
	ShowSessionTokens     bool     `json:"showSessionTokens"`
	ShowOutputStyle       bool     `json:"showOutputStyle"`
	ShowSessionStartDate  bool     `json:"showSessionStartDate"`
	ShowLastResponseAt    bool     `json:"showLastResponseAt"`
	ShowCompactions       bool     `json:"showCompactions"`
	MergeGroups           [][]string `json:"mergeGroups"`
	AutocompactBuffer     string   `json:"autocompactBuffer"` // enabled | disabled
	ContextWarningThreshold  float64 `json:"contextWarningThreshold"`
	ContextCriticalThreshold float64 `json:"contextCriticalThreshold"`
	UsageThreshold        float64  `json:"usageThreshold"`
	SevenDayThreshold     float64  `json:"sevenDayThreshold"`
	EnvironmentThreshold  float64  `json:"environmentThreshold"`
	ExternalUsagePath     string   `json:"externalUsagePath"`
	ExternalUsageWritePath string  `json:"externalUsageWritePath"`
	ExternalUsageFreshnessMs int   `json:"externalUsageFreshnessMs"`
	ModelFormat           string   `json:"modelFormat"` // full | compact | short
	ModelOverride         string   `json:"modelOverride"`
	ModelSource           string   `json:"modelSource"` // auto | stdin | transcript
	ShowProvider          bool     `json:"showProvider"`
	ProviderName          string   `json:"providerName"`
	CustomLine            string   `json:"customLine"`
	CustomLinePosition    string   `json:"customLinePosition"` // first | last
	TimeFormat            string   `json:"timeFormat"`
	ShowAdvisor           bool     `json:"showAdvisor"`
	AdvisorOverride       string   `json:"advisorOverride"`
	AutoCompactWindow     *int     `json:"autoCompactWindow"`
}

// Colors ports the color overrides. Values may be a named preset, a 256-color
// index, or a #rrggbb hex string, so they are decoded as `any`.
type Colors struct {
	Context      any    `json:"context"`
	Usage        any    `json:"usage"`
	Warning      any    `json:"warning"`
	UsageWarning any    `json:"usageWarning"`
	Critical     any    `json:"critical"`
	Model        any    `json:"model"`
	Project      any    `json:"project"`
	Git          any    `json:"git"`
	GitBranch    any    `json:"gitBranch"`
	Label        any    `json:"label"`
	Custom       any    `json:"custom"`
	BarFilled    string `json:"barFilled"`
	BarEmpty     string `json:"barEmpty"`
}

// HudConfig is the fully-resolved configuration.
type HudConfig struct {
	Language      string    `json:"language"`
	LineLayout    string    `json:"lineLayout"` // compact | expanded
	ShowSeparators bool     `json:"showSeparators"`
	PathLevels    int       `json:"pathLevels"`
	MaxWidth      *int      `json:"maxWidth"`
	ForceMaxWidth bool      `json:"forceMaxWidth"`
	ElementOrder  []string  `json:"elementOrder"`
	GitStatus     GitConfig `json:"gitStatus"`
	Display       Display   `json:"display"`
	Colors        Colors    `json:"colors"`
}

// DefaultElementOrder mirrors DEFAULT_ELEMENT_ORDER.
var DefaultElementOrder = []string{
	"project", "addedDirs", "context", "usage", "promptCache", "memory",
	"environment", "tools", "skills", "mcp", "agents", "todos", "sessionTime",
}

var knownElements = func() map[string]bool {
	m := map[string]bool{}
	for _, e := range DefaultElementOrder {
		m[e] = true
	}
	return m
}()

// Default returns a fresh copy of DEFAULT_CONFIG.
func Default() HudConfig {
	order := make([]string, len(DefaultElementOrder))
	copy(order, DefaultElementOrder)
	return HudConfig{
		Language:       "en",
		LineLayout:     "expanded",
		ShowSeparators: false,
		PathLevels:     1,
		MaxWidth:       nil,
		ForceMaxWidth:  false,
		ElementOrder:   order,
		GitStatus: GitConfig{
			Enabled:         true,
			ShowDirty:       true,
			ShowAheadBehind: false,
			ShowFileStats:   false,
			BranchOverflow:  "truncate",
		},
		Display: Display{
			ShowModel: true, ShowProject: true, ShowAddedDirs: true,
			AddedDirsLayout: "inline", ShowContextBar: true, ContextValue: "percent",
			ShowTokenBreakdown: true, ShowUsage: true, UsageValue: "percent",
			UsageBarEnabled: true, ShowScopedUsage: true, ShowResetLabel: true, ToolsMaxVisible: 4,
			AuthUserLength: 8, PromptCacheTtlSeconds: 300,
			MergeGroups:              [][]string{{"context", "usage"}},
			AutocompactBuffer:        "enabled",
			ContextWarningThreshold:  70,
			ContextCriticalThreshold: 85,
			SevenDayThreshold:        80,
			ExternalUsageFreshnessMs: 300000,
			ModelFormat:              "full",
			ModelSource:              "stdin",
			CustomLinePosition:       "last",
			TimeFormat:               "relative",
		},
		Colors: Colors{
			Context: "green", Usage: "brightBlue", Warning: "yellow",
			UsageWarning: "brightMagenta", Critical: "red", Model: "cyan",
			Project: "yellow", Git: "magenta", GitBranch: "cyan", Label: "dim",
			Custom: float64(208), BarFilled: "█", BarEmpty: "░",
		},
	}
}

// Load reads config.json and merges it over the defaults. Missing file or
// invalid JSON falls back to defaults (matching loadConfig()).
func Load() HudConfig {
	cfg := Default()
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	// Unmarshal on top of defaults: only keys present in the file are overwritten.
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default()
	}
	normalize(&cfg)
	return cfg
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// normalize clamps/validates the merged config the way the TS validators do.
func normalize(c *HudConfig) {
	if !validLanguage(c.Language) {
		c.Language = "en"
	}
	if c.LineLayout != "compact" && c.LineLayout != "expanded" {
		c.LineLayout = "expanded"
	}
	if c.PathLevels < 1 || c.PathLevels > 3 {
		c.PathLevels = 1
	}
	if c.MaxWidth != nil && *c.MaxWidth <= 0 {
		c.MaxWidth = nil
	}
	c.ElementOrder = normalizeElementOrder(c.ElementOrder)
	c.Display.MergeGroups = normalizeMergeGroups(c.Display.MergeGroups)

	d := &c.Display
	d.ContextWarningThreshold = clamp(d.ContextWarningThreshold, 0, 100)
	d.ContextCriticalThreshold = clamp(d.ContextCriticalThreshold, 0, 100)
	d.UsageThreshold = clamp(d.UsageThreshold, 0, 100)
	d.SevenDayThreshold = clamp(d.SevenDayThreshold, 0, 100)
	d.EnvironmentThreshold = clamp(d.EnvironmentThreshold, 0, 100)
	if d.ToolsMaxVisible < 0 {
		d.ToolsMaxVisible = 4
	}
	if d.ToolNameMaxLength < 0 {
		d.ToolNameMaxLength = 0
	}
	if !oneOf(d.ModelFormat, "full", "compact", "short") {
		d.ModelFormat = "full"
	}
	if !oneOf(d.ModelSource, "auto", "stdin", "transcript") {
		d.ModelSource = "stdin"
	}
	if !oneOf(d.ContextValue, "percent", "tokens", "remaining", "both") {
		d.ContextValue = "percent"
	}
	if !oneOf(d.UsageValue, "percent", "remaining") {
		d.UsageValue = "percent"
	}
	if c.Colors.BarFilled == "" {
		c.Colors.BarFilled = "█"
	}
	if c.Colors.BarEmpty == "" {
		c.Colors.BarEmpty = "░"
	}
}

func validLanguage(l string) bool {
	return oneOf(l, "en", "zh", "zh-Hans", "zh-Hant", "zh-TW")
}

func oneOf(v string, opts ...string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}

func normalizeElementOrder(in []string) []string {
	if len(in) == 0 {
		out := make([]string, len(DefaultElementOrder))
		copy(out, DefaultElementOrder)
		return out
	}
	seen := map[string]bool{}
	out := []string{}
	for _, e := range in {
		if !knownElements[e] || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	if len(out) == 0 {
		out = make([]string, len(DefaultElementOrder))
		copy(out, DefaultElementOrder)
	}
	return out
}

func normalizeMergeGroups(in [][]string) [][]string {
	if in == nil {
		return [][]string{{"context", "usage"}}
	}
	if len(in) == 0 {
		return [][]string{}
	}
	used := map[string]bool{}
	var groups [][]string
	for _, g := range in {
		seen := map[string]bool{}
		var ng []string
		for _, e := range g {
			if !knownElements[e] || seen[e] || used[e] {
				continue
			}
			seen[e] = true
			ng = append(ng, e)
		}
		if len(ng) >= 2 {
			for _, e := range ng {
				used[e] = true
			}
			groups = append(groups, ng)
		}
	}
	if len(groups) == 0 {
		return [][]string{{"context", "usage"}}
	}
	return groups
}
