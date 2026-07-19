package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/jarrodwatts/claude-hud-go/internal/config"
	"github.com/jarrodwatts/claude-hud-go/internal/i18n"
	"github.com/jarrodwatts/claude-hud-go/internal/stdinp"
	"github.com/jarrodwatts/claude-hud-go/internal/types"
)

const (
	fiveHourWindow = 5 * time.Hour
	sevenDayWindow = 7 * 24 * time.Hour
)

// --- project line (model + project + git + extras) ---

func (c *Context) renderProjectLine() string {
	d := c.Config.Display
	cl := c.Config.Colors
	var parts []string

	if d.CustomLine != "" && d.CustomLinePosition == "first" {
		parts = append(parts, colCustom(d.CustomLine, cl))
	}
	if d.ShowModel {
		name := stdinp.FormatModelName(
			stdinp.ResolveModelName(c.Stdin, c.Transcript.LastAssistantModel, d.ModelSource),
			d.ModelFormat, d.ModelOverride)
		if d.ShowProvider {
			if p := stdinp.ProviderLabel(c.Stdin); p != "" {
				name = p + " " + name
			} else if d.ProviderName != "" {
				name = d.ProviderName + " " + name
			}
		}
		parts = append(parts, colModel("["+name+"]", cl))
	}

	var projectPart string
	if d.ShowProject && c.Stdin != nil && c.Stdin.Cwd != "" {
		segs := splitPath(c.Stdin.Cwd)
		levels := c.Config.PathLevels
		var p string
		if len(segs) > 0 {
			if levels > len(segs) {
				levels = len(segs)
			}
			p = strings.Join(segs[len(segs)-levels:], "/")
		} else {
			p = "/"
		}
		projectPart = safeHyperlink(getFileHref(c.Stdin.Cwd), colProject(p, cl))
	}

	var gitPart string
	if c.Config.GitStatus.Enabled && c.Git != nil {
		branch := c.Git.Branch
		if c.Config.GitStatus.ShowDirty && c.Git.Dirty {
			branch += "*"
		}
		inner := []string{colGitBranch(branch, cl)}
		if c.Config.GitStatus.ShowAheadBehind {
			if c.Git.Ahead > 0 {
				inner = append(inner, colGitBranch(fmt.Sprintf("↑%d", c.Git.Ahead), cl))
			}
			if c.Git.Behind > 0 {
				inner = append(inner, colGitBranch(fmt.Sprintf("↓%d", c.Git.Behind), cl))
			}
		}
		if c.Config.GitStatus.ShowFileStats && (c.Git.Insertions > 0 || c.Git.Deletions > 0) {
			var dp []string
			if c.Git.Insertions > 0 {
				dp = append(dp, green(fmt.Sprintf("+%d", c.Git.Insertions)))
			}
			if c.Git.Deletions > 0 {
				dp = append(dp, red(fmt.Sprintf("-%d", c.Git.Deletions)))
			}
			inner = append(inner, "["+strings.Join(dp, " ")+"]")
		}
		gitPart = colGit("git:(", cl) + strings.Join(inner, " ") + colGit(")", cl)
	}

	switch {
	case projectPart != "" && gitPart != "":
		if c.Config.GitStatus.BranchOverflow == "wrap" {
			parts = append(parts, projectPart, gitPart)
		} else {
			parts = append(parts, projectPart+" "+gitPart)
		}
	case projectPart != "":
		parts = append(parts, projectPart)
	case gitPart != "":
		parts = append(parts, gitPart)
	}

	if d.ShowAdvisor {
		advisor := d.AdvisorOverride
		if advisor == "" {
			advisor = c.Transcript.AdvisorModel
		}
		if advisor != "" {
			parts = append(parts, colLabel(i18n.T("label.advisor")+": "+advisor, cl))
		}
	}
	if d.ShowSessionName && c.Transcript.SessionName != "" {
		parts = append(parts, colLabel(c.Transcript.SessionName, cl))
	}
	if d.ShowDuration && c.SessionDuration != "" {
		parts = append(parts, colLabel("⏱️  "+c.SessionDuration, cl))
	}
	if d.ShowCost && c.Stdin != nil && c.Stdin.Cost != nil && c.Stdin.Cost.TotalCostUSD != nil {
		parts = append(parts, colLabel(fmt.Sprintf("%s $%.2f", i18n.T("label.cost"), *c.Stdin.Cost.TotalCostUSD), cl))
	}
	if d.ShowOutputStyle && c.OutputStyle != "" {
		parts = append(parts, colLabel(c.OutputStyle, cl))
	}
	if d.CustomLine != "" && d.CustomLinePosition == "last" {
		parts = append(parts, colCustom(d.CustomLine, cl))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " │ ")
}

// --- context (identity) line ---

func (c *Context) renderContextLine(labelText string) string {
	d := c.Config.Display
	cl := c.Config.Colors
	acw := d.AutoCompactWindow
	raw := stdinp.ContextPercent(c.Stdin, acw)
	buffered := stdinp.BufferedPercent(c.Stdin, acw)
	percent := buffered
	if d.AutocompactBuffer == "disabled" {
		percent = raw
	}
	warn := d.ContextWarningThreshold
	crit := d.ContextCriticalThreshold
	val := c.formatContextValue(percent, d.ContextValue)
	valDisplay := getContextColor(float64(percent), cl, warn, crit) + val + Reset

	var line string
	if d.ShowContextBar {
		line = labelText + " " + coloredBar(float64(percent), adaptiveBarWidth(), cl, warn, crit) + " " + valDisplay
	} else {
		line = labelText + " " + valDisplay
	}
	critThreshold := d.ContextCriticalThreshold
	if critThreshold == 0 {
		critThreshold = 85
	}
	if d.ShowTokenBreakdown && float64(percent) >= critThreshold {
		if c.Stdin != nil && c.Stdin.ContextWindow != nil && c.Stdin.ContextWindow.CurrentUsage != nil {
			u := c.Stdin.ContextWindow.CurrentUsage
			input := formatTokens(u.InputTokens)
			cache := formatTokens(u.CacheCreationInputTokens + u.CacheReadInputTokens)
			line += colLabel(fmt.Sprintf(" (%s: %s, %s: %s)",
				i18n.T("format.in"), input, i18n.T("format.cache"), cache), cl)
		}
	}
	return line
}

// --- usage line (5h + weekly windows) ---

func (c *Context) renderUsageLine(labelText string) string {
	d := c.Config.Display
	cl := c.Config.Colors
	if !d.ShowUsage || c.Usage == nil || stdinp.ShouldHideUsage(c.Stdin) {
		return ""
	}
	u := c.Usage
	now := time.Now()

	// Model-scoped weekly windows (e.g. Fable) rendered as extra segments.
	scopedSuffix := ""
	if d.ShowScopedUsage && len(u.ScopedWindows) > 0 {
		barW := adaptiveBarWidth()
		var segs []string
		for _, w := range u.ScopedWindows {
			segs = append(segs, c.usageWindowPart(w.Label, w.Percent, w.ResetAt, barW, true))
		}
		scopedSuffix = " | " + strings.Join(segs, " | ")
	}

	if u.IsLimitReached() {
		var reset string
		if u.FiveHour != nil && *u.FiveHour == 100 {
			reset = formatResetTime(u.FiveHourResetAt, timeFmt(d.TimeFormat), now)
		} else {
			reset = formatResetTime(u.SevenDayResetAt, timeFmt(d.TimeFormat), now)
		}
		suffix := ""
		if reset != "" {
			if d.ShowResetLabel {
				suffix = fmt.Sprintf(" (%s %s)", i18n.T("format.resetsIn"), reset)
			} else {
				suffix = fmt.Sprintf(" (%s)", reset)
			}
		}
		return labelText + " " + colCritical("⚠ "+i18n.T("status.limitReached")+suffix, cl) + scopedSuffix
	}

	if u.FiveHour == nil && u.SevenDay == nil {
		if scopedSuffix != "" {
			return labelText + " " + strings.TrimPrefix(scopedSuffix, " | ")
		}
		if u.BalanceLabel != "" {
			return labelText + " " + u.BalanceLabel
		}
		return ""
	}
	barW := adaptiveBarWidth()
	if u.FiveHour == nil && u.SevenDay != nil {
		return labelText + " " + c.usageWindowPart(i18n.T("label.weekly"), u.SevenDay, u.SevenDayResetAt, barW, true) + scopedSuffix
	}
	fivePart := c.usageWindowPart("5h", u.FiveHour, u.FiveHourResetAt, barW, false)
	sevenThreshold := d.SevenDayThreshold
	if u.SevenDay != nil && float64(*u.SevenDay) >= sevenThreshold {
		sevenPart := c.usageWindowPart(i18n.T("label.weekly"), u.SevenDay, u.SevenDayResetAt, barW, true)
		return labelText + " " + fivePart + " | " + sevenPart + scopedSuffix
	}
	return labelText + " " + fivePart + scopedSuffix
}

func (c *Context) usageWindowPart(lbl string, percent *int, resetAt *time.Time, barW int, forceLabel bool) string {
	d := c.Config.Display
	cl := c.Config.Colors
	now := time.Now()
	usageDisplay := "--"
	if percent != nil {
		p := *percent
		if d.UsageValue == "remaining" {
			p = 100 - p
			if p < 0 {
				p = 0
			}
		}
		usageDisplay = getQuotaColor(float64(*percent), cl) + fmt.Sprintf("%d%%", p) + Reset
	} else {
		usageDisplay = colLabel("--", cl)
	}
	reset := formatResetTime(resetAt, timeFmt(d.TimeFormat), now)
	resetSuffix := ""
	if reset != "" {
		if d.ShowResetLabel {
			resetSuffix = fmt.Sprintf("(%s %s)", i18n.T("format.resetsIn"), reset)
		} else {
			resetSuffix = fmt.Sprintf("(%s)", reset)
		}
	}
	styledLabel := colLabel(lbl, cl)
	pv := 0
	if percent != nil {
		pv = *percent
	}
	if d.UsageBarEnabled {
		body := quotaBar(float64(pv), barW, cl) + " " + usageDisplay
		if resetSuffix != "" {
			body += " " + resetSuffix
		}
		if forceLabel {
			return styledLabel + " " + body
		}
		return body
	}
	if resetSuffix != "" {
		return styledLabel + " " + usageDisplay + " " + resetSuffix
	}
	return styledLabel + " " + usageDisplay
}

func timeFmt(v string) string {
	if v == "absolute" {
		return "absolute"
	}
	return "relative"
}

// --- environment line ---

func (c *Context) renderEnvironmentLine() string {
	if !c.Config.Display.ShowConfigCounts {
		return ""
	}
	cl := c.Config.Colors
	var parts []string
	if c.Counts.ClaudeMd > 0 {
		parts = append(parts, colLabel(fmt.Sprintf("%d CLAUDE.md", c.Counts.ClaudeMd), cl))
	}
	if c.Counts.Rules > 0 {
		parts = append(parts, colLabel(fmt.Sprintf("%d %s", c.Counts.Rules, i18n.T("label.rules")), cl))
	}
	if c.Counts.Mcp > 0 {
		parts = append(parts, colLabel(fmt.Sprintf("%d MCP", c.Counts.Mcp), cl))
	}
	if c.Counts.Hooks > 0 {
		parts = append(parts, colLabel(fmt.Sprintf("%d %s", c.Counts.Hooks, i18n.T("label.hooks")), cl))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " │ ")
}

// --- tools line ---

func (c *Context) renderToolsLine() string {
	if !c.Config.Display.ShowTools {
		return ""
	}
	cl := c.Config.Colors
	tools := c.Transcript.Tools
	// Suppress Skill entries when the skills line is enabled.
	skillsSeparate := c.Config.Display.ShowSkills
	var running []types.ToolEntry
	completed := map[string]int{}
	var completedOrder []string
	for _, t := range tools {
		if skillsSeparate && t.Name == "Skill" {
			continue
		}
		if t.Status == "running" {
			running = append(running, t)
		} else {
			if _, ok := completed[t.Name]; !ok {
				completedOrder = append(completedOrder, t.Name)
			}
			completed[t.Name]++
		}
	}
	var parts []string
	for _, t := range running {
		s := "◐ " + t.Name
		if t.Target != "" {
			s += ": " + baseName(t.Target)
		}
		parts = append(parts, cyan(s))
	}
	for _, name := range completedOrder {
		n := completed[name]
		if n > 1 {
			parts = append(parts, dim(fmt.Sprintf("✓ %s ×%d", name, n)))
		} else {
			parts = append(parts, dim("✓ "+name))
		}
	}
	max := c.Config.Display.ToolsMaxVisible
	if max > 0 && len(parts) > max {
		parts = parts[:max]
	}
	if len(parts) == 0 {
		return ""
	}
	_ = cl
	return strings.Join(parts, " | ")
}

// --- agents line ---

func (c *Context) renderAgentsLine() string {
	if !c.Config.Display.ShowAgents {
		return ""
	}
	now := time.Now()
	var parts []string
	for _, a := range c.Transcript.Agents {
		icon := "◐"
		if a.Status == "completed" {
			icon = "✓"
		}
		s := icon + " " + a.Type
		if a.Model != "" {
			s += " [" + a.Model + "]"
		}
		if a.Description != "" {
			s += ": " + a.Description
		}
		if a.Status == "running" {
			s += " (" + elapsed(a.StartTime, now) + ")"
		}
		if a.Status == "running" {
			parts = append(parts, cyan(s))
		} else {
			parts = append(parts, dim(s))
		}
	}
	return strings.Join(parts, " | ")
}

// --- todos line ---

func (c *Context) renderTodosLine() string {
	if !c.Config.Display.ShowTodos {
		return ""
	}
	cl := c.Config.Colors
	todos := c.Transcript.Todos
	if len(todos) == 0 {
		return ""
	}
	completed := 0
	var current string
	for _, t := range todos {
		if t.Status == "completed" {
			completed++
		}
		if t.Status == "in_progress" && current == "" {
			current = t.Content
		}
	}
	if completed == len(todos) {
		return dim(i18n.T("status.allTodosComplete"))
	}
	if current == "" && len(todos) > 0 {
		current = todos[0].Content
	}
	return colProject("▸ ", cl) + current + dim(fmt.Sprintf(" (%d/%d)", completed, len(todos)))
}

// --- skills / mcp lines ---

func (c *Context) renderSkillsLine() string {
	if !c.Config.Display.ShowSkills || len(c.Transcript.Skills) == 0 {
		return ""
	}
	var parts []string
	for _, s := range c.Transcript.Skills {
		parts = append(parts, dim("◆ "+s))
	}
	return strings.Join(parts, " | ")
}

func (c *Context) renderMcpLine() string {
	if !c.Config.Display.ShowMcp || len(c.Transcript.McpServers) == 0 {
		return ""
	}
	var parts []string
	for _, s := range c.Transcript.McpServers {
		parts = append(parts, dim("⚡ "+s))
	}
	return strings.Join(parts, " | ")
}

// --- session time line ---

func (c *Context) renderSessionTimeLine() string {
	d := c.Config.Display
	cl := c.Config.Colors
	var parts []string
	if d.ShowSessionStartDate && c.Transcript.SessionStart != nil {
		parts = append(parts, colLabel(i18n.T("label.sessionStarted")+": "+c.Transcript.SessionStart.Format("15:04"), cl))
	}
	if d.ShowLastResponseAt && c.Transcript.LastAssistantResponseAt != nil {
		parts = append(parts, colLabel(i18n.T("label.lastReply")+": "+
			formatResetTimeAgo(c.Transcript.LastAssistantResponseAt, time.Now()), cl))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " │ ")
}

func formatResetTimeAgo(t *time.Time, now time.Time) string {
	if t == nil {
		return ""
	}
	d := now.Sub(*t)
	mins := int(d.Minutes())
	if mins < 1 {
		return i18n.T("format.justNow")
	}
	var val string
	if mins < 60 {
		val = fmt.Sprintf("%dm", mins)
	} else {
		val = fmt.Sprintf("%dh", mins/60)
	}
	return i18n.Interpolate(i18n.T("format.relativeTime"), map[string]any{"value": val})
}

// --- session tokens / compactions ---

func (c *Context) renderSessionTokensLine() string {
	if !c.Config.Display.ShowSessionTokens || c.Transcript.SessionTokens == nil {
		return ""
	}
	cl := c.Config.Colors
	t := c.Transcript.SessionTokens
	return colLabel(fmt.Sprintf("%s: %s %s / %s %s",
		i18n.T("label.tokens"),
		i18n.T("format.in"), formatTokens(t.InputTokens),
		i18n.T("format.out"), formatTokens(t.OutputTokens)), cl)
}

func (c *Context) renderCompactionsLine() string {
	if !c.Config.Display.ShowCompactions || c.Transcript.CompactionCount == 0 {
		return ""
	}
	cl := c.Config.Colors
	return colLabel(fmt.Sprintf("%s: %d", i18n.T("label.compactions"), c.Transcript.CompactionCount), cl)
}

func (c *Context) renderMemoryLine(labelText string) string {
	if !c.Config.Display.ShowMemoryUsage || c.Memory == nil {
		return ""
	}
	cl := c.Config.Colors
	return labelText + " " + coloredBar(c.Memory.UsedPercent, adaptiveBarWidth(), cl, 70, 85) +
		fmt.Sprintf(" %d%%", int(c.Memory.UsedPercent))
}

// --- helpers ---

func splitPath(p string) []string {
	fields := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	return fields
}

func baseName(p string) string {
	fields := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	if len(fields) == 0 {
		return p
	}
	return fields[len(fields)-1]
}

func elapsed(start, now time.Time) string {
	d := now.Sub(start)
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	return fmt.Sprintf("%dm %ds", m, s%60)
}

func progressLabel(key string, c config.Colors) string {
	return colLabel(i18n.T(key), c)
}
