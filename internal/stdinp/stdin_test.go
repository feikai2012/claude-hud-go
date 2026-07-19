package stdinp

import (
	"encoding/json"
	"testing"

	"github.com/jarrodwatts/claude-hud-go/internal/types"
)

func mk(input, size int, usedPct *float64) *types.StdinData {
	return &types.StdinData{
		ContextWindow: &types.ContextWindow{
			ContextWindowSize: size,
			CurrentUsage:      &types.CurrentUsage{InputTokens: input},
			UsedPercentage:    usedPct,
		},
	}
}

func TestContextPercent(t *testing.T) {
	// 45000 / 200000 = 22.5% → 23 (rounded).
	if got := ContextPercent(mk(45000, 200000, nil), nil); got != 23 {
		t.Fatalf("ContextPercent = %d, want 23", got)
	}
	// Native percentage wins when > 0.
	p := 40.0
	if got := ContextPercent(mk(45000, 200000, &p), nil); got != 40 {
		t.Fatalf("native ContextPercent = %d, want 40", got)
	}
	// Native 0 is ignored (falls through to token math).
	z := 0.0
	if got := ContextPercent(mk(45000, 200000, &z), nil); got != 23 {
		t.Fatalf("native-zero ContextPercent = %d, want 23", got)
	}
}

func TestBufferedPercent(t *testing.T) {
	// Matches the TS autocompact-buffer formula: 45k/200k → 29%.
	if got := BufferedPercent(mk(45000, 200000, nil), nil); got != 29 {
		t.Fatalf("BufferedPercent = %d, want 29", got)
	}
}

func TestModelName(t *testing.T) {
	cases := map[string]*types.StdinData{
		"Opus 4.8":      {Model: &types.Model{DisplayName: "Opus 4.8"}},
		"Claude Opus":   {Model: &types.Model{ID: "opusplan"}},
		"Unknown":       {},
		"Claude Sonnet 4": {Model: &types.Model{ID: "anthropic.claude-sonnet-4-20250101-v1:0"}},
	}
	for want, s := range cases {
		if got := ModelName(s); got != want {
			t.Errorf("ModelName(%+v) = %q, want %q", s.Model, got, want)
		}
	}
}

func TestGetUsageFromStdin(t *testing.T) {
	raw := `{"rate_limits":{"five_hour":{"used_percentage":25},"seven_day":{"used_percentage":85}}}`
	var s types.StdinData
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	u := GetUsageFromStdin(&s)
	if u == nil || u.FiveHour == nil || *u.FiveHour != 25 || u.SevenDay == nil || *u.SevenDay != 85 {
		t.Fatalf("unexpected usage: %+v", u)
	}
}
