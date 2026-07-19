// Package stdinp ports src/stdin.ts: reading Claude Code's JSON from stdin plus
// the context-percentage, usage, and model-name derivations.
package stdinp

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jarrodwatts/claude-hud-go/internal/config"
	"github.com/jarrodwatts/claude-hud-go/internal/types"
)

const (
	// readDeadline is a safety net so the process never hangs if stdin is a pipe
	// that is held open with no data. Claude Code closes stdin after writing the
	// JSON, so ReadAll normally returns well within this. It must be generous
	// enough to absorb process-spawn overhead (a 250ms first-byte window is too
	// tight on Windows, where cmd.exe/PowerShell startup can exceed it before the
	// child even sees the pipe — that produced a spurious "Initializing...").
	readDeadline  = 2 * time.Second
	maxStdinBytes = 256 * 1024
)

// Read returns the parsed stdin payload, or nil when stdin is an interactive
// TTY (no piped data). Mirrors readStdin() from stdin.ts.
func Read() *types.StdinData {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return nil
	}
	// A character device is an interactive TTY (setup verification path). A pipe
	// or regular file (what Claude Code uses) is not, so we read it.
	if fi.Mode()&os.ModeCharDevice != 0 {
		return nil
	}

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes+1))
		ch <- result{b, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil && r.err != io.EOF {
			return nil
		}
		return parse(r.data)
	case <-time.After(readDeadline):
		return nil
	}
}

func parse(b []byte) *types.StdinData {
	// Strip a leading UTF-8 BOM (some Windows producers prepend one).
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" || len(b) > maxStdinBytes {
		return nil
	}
	var d types.StdinData
	if err := json.Unmarshal([]byte(trimmed), &d); err != nil {
		return nil
	}
	return &d
}

// TotalTokens = input + cache_creation + cache_read (matches getTotalTokens).
func TotalTokens(s *types.StdinData) int {
	if s == nil || s.ContextWindow == nil || s.ContextWindow.CurrentUsage == nil {
		return 0
	}
	u := s.ContextWindow.CurrentUsage
	return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// nativePercent returns the rounded native used_percentage only when > 0.
func nativePercent(s *types.StdinData) (int, bool) {
	if s == nil || s.ContextWindow == nil || s.ContextWindow.UsedPercentage == nil {
		return 0, false
	}
	p := *s.ContextWindow.UsedPercentage
	if math.IsNaN(p) || p <= 0 {
		return 0, false
	}
	return int(math.Round(math.Min(100, math.Max(0, p)))), true
}

// ContextPercent mirrors getContextPercent (no buffer).
func ContextPercent(s *types.StdinData, autoCompactWindow *int) int {
	if autoCompactWindow != nil && *autoCompactWindow > 0 {
		return minInt(100, int(math.Round(float64(TotalTokens(s))/float64(*autoCompactWindow)*100)))
	}
	if p, ok := nativePercent(s); ok {
		return p
	}
	size := contextSize(s)
	if size <= 0 {
		return 0
	}
	return minInt(100, int(math.Round(float64(TotalTokens(s))/float64(size)*100)))
}

// BufferedPercent mirrors getBufferedPercent (autocompact buffer fallback).
func BufferedPercent(s *types.StdinData, autoCompactWindow *int) int {
	if autoCompactWindow != nil && *autoCompactWindow > 0 {
		return minInt(100, int(math.Round(float64(TotalTokens(s))/float64(*autoCompactWindow)*100)))
	}
	if p, ok := nativePercent(s); ok {
		return p
	}
	size := contextSize(s)
	if size <= 0 {
		return 0
	}
	total := float64(TotalTokens(s))
	rawRatio := total / float64(size)
	const low, high = 0.05, 0.50
	scale := math.Min(1, math.Max(0, (rawRatio-low)/(high-low)))
	buffer := float64(size) * config.AutocompactBufferPercent * scale
	return minInt(100, int(math.Round((total+buffer)/float64(size)*100)))
}

func contextSize(s *types.StdinData) int {
	if s == nil || s.ContextWindow == nil {
		return 0
	}
	return s.ContextWindow.ContextWindowSize
}

var enterpriseAliasLabels = map[string]string{
	"opusplan":   "Claude Opus",
	"sonnetplan": "Claude Sonnet",
	"haikuplan":  "Claude Haiku",
}

// ModelName mirrors getModelName.
func ModelName(s *types.StdinData) string {
	if s == nil || s.Model == nil {
		return "Unknown"
	}
	if dn := strings.TrimSpace(s.Model.DisplayName); dn != "" {
		return dn
	}
	id := strings.TrimSpace(s.Model.ID)
	if id == "" {
		return "Unknown"
	}
	if label, ok := enterpriseAliasLabels[strings.ToLower(id)]; ok {
		return label
	}
	if label := normalizeBedrockModelLabel(id); label != "" {
		return label
	}
	return id
}

func isClaudeModel(model string) bool {
	if model == "" {
		return true
	}
	l := strings.ToLower(model)
	return strings.HasPrefix(l, "claude-") || strings.HasPrefix(l, "anthropic.")
}

// ResolveModelName mirrors resolveModelName (modelSource: auto|stdin|transcript).
func ResolveModelName(s *types.StdinData, transcriptModel, modelSource string) string {
	stdinModel := ModelName(s)
	tm := sanitizeModel(transcriptModel)
	if modelSource == "stdin" || tm == "" {
		return stdinModel
	}
	if modelSource == "transcript" {
		return tm
	}
	if isClaudeModel(tm) {
		return stdinModel
	}
	return tm
}

// sanitizeModel trims terminal controls and caps length (see model-source.ts).
var ctrlRe = regexp.MustCompile(`[\x00-\x1f\x7f]`)

func sanitizeModel(m string) string {
	m = ctrlRe.ReplaceAllString(m, "")
	m = strings.TrimSpace(m)
	if len(m) > 80 {
		m = m[:80]
	}
	return m
}

func isBedrockModelID(id string) bool {
	return id != "" && strings.Contains(strings.ToLower(id), "anthropic.claude-")
}

func isEnterpriseModelID(id string) bool {
	switch strings.ToLower(id) {
	case "opusplan", "sonnetplan", "haikuplan":
		return true
	}
	return false
}

// ProviderLabel mirrors getProviderLabel.
func ProviderLabel(s *types.StdinData) string {
	if os.Getenv("CLAUDE_CODE_USE_BEDROCK") == "1" {
		return "Bedrock"
	}
	if os.Getenv("CLAUDE_CODE_USE_VERTEX") == "1" {
		return "Vertex"
	}
	if s != nil && s.Model != nil && isEnterpriseModelID(s.Model.ID) {
		return "Enterprise"
	}
	return ""
}

// ShouldHideUsage mirrors shouldHideUsage (Bedrock hides subscriber usage).
func ShouldHideUsage(s *types.StdinData) bool {
	if ProviderLabel(s) == "Bedrock" {
		return true
	}
	return s != nil && s.Model != nil && isBedrockModelID(s.Model.ID)
}

var contextSuffixRe = regexp.MustCompile(`(?i)\s*\([^)]*\bcontext\b[^)]*\)`)
var claudePrefixRe = regexp.MustCompile(`(?i)^Claude\s+`)

// StripContextSuffix mirrors stripContextSuffix.
func StripContextSuffix(name string) string {
	return strings.TrimSpace(contextSuffixRe.ReplaceAllString(name, ""))
}

// FormatModelName mirrors formatModelName.
func FormatModelName(name, format, override string) string {
	if override != "" {
		return override
	}
	if format == "" || format == "full" {
		return name
	}
	result := StripContextSuffix(name)
	if format == "short" {
		result = claudePrefixRe.ReplaceAllString(result, "")
	}
	return result
}

// GetUsageFromStdin mirrors getUsageFromStdin.
func GetUsageFromStdin(s *types.StdinData) *types.UsageData {
	if s == nil || s.RateLimits == nil {
		return nil
	}
	rl := s.RateLimits
	var five, seven *int
	var fiveReset, sevenReset *time.Time
	if rl.FiveHour != nil {
		five = rateLimitPercent(rl.FiveHour.UsedPercentage)
		fiveReset = rateLimitResetAt(rl.FiveHour.ResetsAt)
	}
	if rl.SevenDay != nil {
		seven = rateLimitPercent(rl.SevenDay.UsedPercentage)
		sevenReset = rateLimitResetAt(rl.SevenDay.ResetsAt)
	}
	scoped := parseScopedWindows(rl.ModelScoped)
	if five == nil && seven == nil && len(scoped) == 0 {
		return nil
	}
	return &types.UsageData{
		FiveHour: five, SevenDay: seven,
		FiveHourResetAt: fiveReset, SevenDayResetAt: sevenReset,
		ScopedWindows: scoped,
	}
}

func rateLimitPercent(v *float64) *int {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	r := int(math.Round(math.Min(100, math.Max(0, *v))))
	return &r
}

func rateLimitResetAt(v *float64) *time.Time {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0 {
		return nil
	}
	t := time.Unix(int64(*v), 0)
	return &t
}

const (
	scopedMaxWindows = 8
	scopedLabelMax   = 64
	scopedResetMax   = 64
)

func parseScopedWindows(in []types.ModelScopedWindow) []types.ScopedUsageWindow {
	var out []types.ScopedUsageWindow
	for _, e := range in {
		if len(out) >= scopedMaxWindows {
			break
		}
		if e.DisplayName == nil {
			continue
		}
		label := strings.TrimSpace(sanitizeModel(*e.DisplayName))
		if len(label) > scopedLabelMax {
			label = label[:scopedLabelMax]
		}
		if label == "" {
			continue
		}
		var percent *int
		if e.Utilization != nil {
			percent = rateLimitPercent(e.Utilization)
			if percent == nil {
				continue
			}
		}
		var resetAt *time.Time
		if e.ResetsAt != nil && len(*e.ResetsAt) <= scopedResetMax {
			if t, err := time.Parse(time.RFC3339, *e.ResetsAt); err == nil {
				resetAt = &t
			}
		}
		out = append(out, types.ScopedUsageWindow{Label: label, Percent: percent, ResetAt: resetAt})
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// normalizeBedrockModelLabel mirrors the Bedrock label normalization.
func normalizeBedrockModelLabel(modelID string) string {
	if !isBedrockModelID(modelID) {
		return ""
	}
	lower := strings.ToLower(modelID)
	const prefix = "anthropic.claude-"
	idx := strings.Index(lower, prefix)
	if idx == -1 {
		return ""
	}
	suffix := lower[idx+len(prefix):]
	suffix = regexp.MustCompile(`-v\d+:\d+$`).ReplaceAllString(suffix, "")
	suffix = regexp.MustCompile(`-\d{8}$`).ReplaceAllString(suffix, "")
	var tokens []string
	for _, t := range strings.Split(suffix, "-") {
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	familyIdx := -1
	for i, t := range tokens {
		if t == "haiku" || t == "sonnet" || t == "opus" {
			familyIdx = i
			break
		}
	}
	if familyIdx == -1 {
		return ""
	}
	family := tokens[familyIdx]
	before := reverse(readNumericVersion(tokens, familyIdx-1, -1))
	after := readNumericVersion(tokens, familyIdx+1, 1)
	parts := after
	if len(before) >= len(after) {
		parts = before
	}
	familyLabel := strings.ToUpper(family[:1]) + family[1:]
	if len(parts) > 0 {
		return "Claude " + familyLabel + " " + strings.Join(parts, ".")
	}
	return "Claude " + familyLabel
}

var numericRe = regexp.MustCompile(`^\d+$`)

func readNumericVersion(tokens []string, start, step int) []string {
	var parts []string
	for i := start; i >= 0 && i < len(tokens); i += step {
		if !numericRe.MatchString(tokens[i]) {
			break
		}
		parts = append(parts, tokens[i])
		if len(parts) == 2 {
			break
		}
	}
	return parts
}

func reverse(in []string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}
