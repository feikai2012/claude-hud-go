package render

import (
	"strings"

	"github.com/jarrodwatts/claude-hud-go/internal/i18n"
	"github.com/jarrodwatts/claude-hud-go/internal/textwidth"
)

var activityElements = map[string]bool{
	"tools": true, "skills": true, "mcp": true, "agents": true, "todos": true,
}

// renderElement produces a single element's line (empty string = omit).
func (c *Context) renderElement(element string) string {
	cl := c.Config.Colors
	switch element {
	case "project":
		return c.renderProjectLine()
	case "context":
		return c.renderContextLine(progressLabel("label.context", cl))
	case "usage":
		return c.renderUsageLine(progressLabel("label.usage", cl))
	case "memory":
		return c.renderMemoryLine(progressLabel("label.approxRam", cl))
	case "environment":
		return c.renderEnvironmentLine()
	case "tools":
		return c.renderToolsLine()
	case "skills":
		return c.renderSkillsLine()
	case "mcp":
		return c.renderMcpLine()
	case "agents":
		return c.renderAgentsLine()
	case "todos":
		return c.renderTodosLine()
	case "sessionTime":
		return c.renderSessionTimeLine()
	case "addedDirs":
		return "" // rendered inline on project line
	case "promptCache":
		return "" // not yet ported
	}
	return ""
}

type outLine struct {
	line       string
	isActivity bool
}

// Render returns the finished HUD lines (each already prefixed with RESET).
func (c *Context) Render() []string {
	i18n.SetLanguage(c.Config.Language)

	termWidth := textwidth.TerminalWidth()

	var rendered []outLine
	if c.Config.LineLayout == "expanded" {
		rendered = c.renderExpanded(termWidth)
	} else {
		if l := c.renderProjectLine(); l != "" {
			rendered = append(rendered, outLine{l, false})
		}
		if l := c.renderContextLine(progressLabel("label.context", c.Config.Colors)); l != "" {
			rendered = append(rendered, outLine{l, false})
		}
		for _, e := range []string{"tools", "skills", "mcp", "agents", "todos"} {
			if l := c.renderElement(e); l != "" {
				rendered = append(rendered, outLine{l, true})
			}
		}
	}

	// Trailing opt-in lines.
	if l := c.renderSessionTokensLine(); l != "" {
		rendered = append(rendered, outLine{l, false})
	}
	if l := c.renderCompactionsLine(); l != "" {
		rendered = append(rendered, outLine{l, false})
	}

	lines := make([]string, 0, len(rendered))
	for _, r := range rendered {
		lines = append(lines, r.line)
	}

	// Separators before the first activity line.
	if c.Config.ShowSeparators {
		lines = insertSeparator(rendered, termWidth)
	}

	// Split embedded newlines, then wrap to width.
	var physical []string
	for _, l := range lines {
		physical = append(physical, strings.Split(l, "\n")...)
	}
	var visible []string
	wrapWidth := termWidth
	for _, l := range physical {
		if wrapWidth > 0 {
			visible = append(visible, textwidth.WrapLineToWidth(l, wrapWidth)...)
		} else {
			visible = append(visible, l)
		}
	}

	out := make([]string, 0, len(visible))
	for _, l := range visible {
		out = append(out, Reset+l)
	}
	return out
}

func (c *Context) renderExpanded(termWidth int) []outLine {
	order := c.Config.ElementOrder
	mergeLookup := buildMergeLookup(c.Config.Display.MergeGroups)
	seen := map[string]bool{}
	var lines []outLine

	for i := 0; i < len(order); i++ {
		el := order[i]
		if seen[el] {
			continue
		}
		if group, ok := mergeLookup[el]; ok {
			seq := collectMergeSequence(order, i, seen, group)
			if len(seq) > 1 {
				i += len(seq) - 1
				var rendered []string
				for _, g := range seq {
					seen[g] = true
					if l := c.renderElement(g); l != "" {
						rendered = append(rendered, l)
					}
				}
				if len(rendered) > 1 {
					combined := strings.Join(rendered, " │ ")
					if termWidth <= 0 || textwidth.VisualLength(combined) <= termWidth {
						lines = append(lines, outLine{combined, anyActivity(seq)})
					} else {
						for _, l := range rendered {
							lines = append(lines, outLine{l, false})
						}
					}
				} else if len(rendered) == 1 {
					lines = append(lines, outLine{rendered[0], false})
				}
				continue
			}
		}
		seen[el] = true
		if l := c.renderElement(el); l != "" {
			lines = append(lines, outLine{l, activityElements[el]})
		}
	}
	return lines
}

func buildMergeLookup(groups [][]string) map[string]map[string]bool {
	lookup := map[string]map[string]bool{}
	for _, g := range groups {
		set := map[string]bool{}
		for _, e := range g {
			set[e] = true
		}
		for _, e := range g {
			if _, ok := lookup[e]; !ok {
				lookup[e] = set
			}
		}
	}
	return lookup
}

func collectMergeSequence(order []string, start int, seen, group map[string]bool) []string {
	var seq []string
	for i := start; i < len(order); i++ {
		el := order[i]
		if seen[el] || !group[el] {
			break
		}
		seq = append(seq, el)
	}
	return seq
}

func anyActivity(seq []string) bool {
	for _, e := range seq {
		if activityElements[e] {
			return true
		}
	}
	return false
}

func insertSeparator(rendered []outLine, termWidth int) []string {
	first := -1
	for i, r := range rendered {
		if r.isActivity {
			first = i
			break
		}
	}
	lines := make([]string, 0, len(rendered)+1)
	for i, r := range rendered {
		if i == first && first > 0 {
			width := 20
			for _, prev := range rendered[:first] {
				if w := textwidth.VisualLength(prev.line); w > width {
					width = w
				}
			}
			if termWidth > 0 && width > termWidth {
				width = termWidth
			}
			lines = append(lines, dim(strings.Repeat("─", width)))
		}
		lines = append(lines, r.line)
	}
	return lines
}
