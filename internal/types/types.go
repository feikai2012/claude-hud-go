// Package types holds the shared data structures for the HUD, ported 1:1 from
// src/types.ts. Optional/nullable JSON fields use pointers so "absent" is
// distinguishable from a zero value (matching the TS `?:` semantics).
package types

import "time"

// StdinData mirrors the JSON blob Claude Code pipes to the statusline on stdin.
type StdinData struct {
	TranscriptPath string        `json:"transcript_path"`
	Cwd            string        `json:"cwd"`
	Workspace      *Workspace    `json:"workspace"`
	Model          *Model        `json:"model"`
	ContextWindow  *ContextWindow `json:"context_window"`
	Cost           *Cost         `json:"cost"`
	RateLimits     *RateLimits   `json:"rate_limits"`
	// Effort is either a bare string (legacy) or an object {level}. Decoded raw.
	Effort any `json:"effort"`
}

type Workspace struct {
	CurrentDir  string   `json:"current_dir"`
	ProjectDir  string   `json:"project_dir"`
	AddedDirs   []string `json:"added_dirs"`
	GitWorktree string   `json:"git_worktree"`
}

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type ContextWindow struct {
	ContextWindowSize  int          `json:"context_window_size"`
	TotalInputTokens   *int         `json:"total_input_tokens"`
	TotalOutputTokens  *int         `json:"total_output_tokens"`
	CurrentUsage       *CurrentUsage `json:"current_usage"`
	UsedPercentage     *float64     `json:"used_percentage"`
	RemainingPercentage *float64    `json:"remaining_percentage"`
}

type CurrentUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type Cost struct {
	TotalCostUSD       *float64 `json:"total_cost_usd"`
	TotalDurationMs    *float64 `json:"total_duration_ms"`
	TotalAPIDurationMs *float64 `json:"total_api_duration_ms"`
	TotalLinesAdded    *float64 `json:"total_lines_added"`
	TotalLinesRemoved  *float64 `json:"total_lines_removed"`
}

type RateLimits struct {
	FiveHour    *RateWindow          `json:"five_hour"`
	SevenDay    *RateWindow          `json:"seven_day"`
	ModelScoped []ModelScopedWindow  `json:"model_scoped"`
}

type RateWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

type ModelScopedWindow struct {
	DisplayName *string  `json:"display_name"`
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

// ToolEntry is a single tool invocation extracted from the transcript.
type ToolEntry struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Target    string     `json:"target,omitempty"`
	Status    string     `json:"status"` // running | completed | error
	StartTime time.Time  `json:"startTime"`
	EndTime   *time.Time `json:"endTime,omitempty"`
}

// AgentEntry is a subagent (Task/Agent) invocation.
type AgentEntry struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Model       string     `json:"model,omitempty"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"` // running | completed
	StartTime   time.Time  `json:"startTime"`
	EndTime     *time.Time `json:"endTime,omitempty"`
	Background  bool       `json:"background,omitempty"`
}

// TodoItem is one entry from the current todo list.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"` // pending | in_progress | completed
}

// SessionTokenUsage is the cumulative token total across the session.
type SessionTokenUsage struct {
	InputTokens         int `json:"inputTokens"`
	OutputTokens        int `json:"outputTokens"`
	CacheCreationTokens int `json:"cacheCreationTokens"`
	CacheReadTokens     int `json:"cacheReadTokens"`
}

// ScopedUsageWindow is one model-scoped weekly quota window (e.g. Fable).
type ScopedUsageWindow struct {
	Label   string
	Percent *int
	ResetAt *time.Time
}

// UsageData holds the resolved subscriber usage windows.
type UsageData struct {
	FiveHour        *int
	SevenDay        *int
	FiveHourResetAt *time.Time
	SevenDayResetAt *time.Time
	BalanceLabel    string
	ScopedWindows   []ScopedUsageWindow
}

// TranscriptData is the parsed result of a session transcript JSONL file.
type TranscriptData struct {
	Tools                  []ToolEntry        `json:"tools"`
	Skills                 []string           `json:"skills"`
	McpServers             []string           `json:"mcpServers"`
	Agents                 []AgentEntry       `json:"agents"`
	Todos                  []TodoItem         `json:"todos"`
	SessionStart           *time.Time         `json:"sessionStart,omitempty"`
	SessionName            string             `json:"sessionName,omitempty"`
	LastAssistantResponseAt *time.Time        `json:"lastAssistantResponseAt,omitempty"`
	SessionTokens          *SessionTokenUsage `json:"sessionTokens,omitempty"`
	LastCompactBoundaryAt  *time.Time         `json:"lastCompactBoundaryAt,omitempty"`
	LastCompactPostTokens  *int               `json:"lastCompactPostTokens,omitempty"`
	CompactionCount        int                `json:"compactionCount,omitempty"`
	AdvisorModel           string             `json:"advisorModel,omitempty"`
	UltracodeActive        *bool              `json:"ultracodeActive,omitempty"`
	LastAssistantModel     string             `json:"lastAssistantModel,omitempty"`
}

// GitStatus is the repository state for the cwd.
type GitStatus struct {
	Branch       string
	Dirty        bool
	Ahead        int
	Behind       int
	FilesChanged int
	Insertions   int
	Deletions    int
}

// MemoryInfo holds system memory stats.
type MemoryInfo struct {
	TotalBytes  uint64
	UsedBytes   uint64
	FreeBytes   uint64
	UsedPercent float64
}

// IsLimitReached reports whether either usage window is at 100%.
func (d UsageData) IsLimitReached() bool {
	return (d.FiveHour != nil && *d.FiveHour == 100) ||
		(d.SevenDay != nil && *d.SevenDay == 100)
}
