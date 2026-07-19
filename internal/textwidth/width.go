// Package textwidth ports src/render/width.ts and the utils/*.ts terminal
// helpers: ANSI-aware, grapheme-aware, CJK-ambiguous-width measurement plus
// truncation and wrapping used by the renderer.
package textwidth

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"

	"github.com/jarrodwatts/claude-hud-go/internal/i18n"
)

// ansiPattern matches a single SGR or OSC escape at the start of a string.
var ansiSingle = regexp.MustCompile(`^(?:\x1b\[[0-9;]*m|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\))`)
var ansiGlobal = regexp.MustCompile(`(?:\x1b\[[0-9;]*m|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\))`)

const reset = "\x1b[0m"
const osc8Close = "\x1b]8;;\x1b\\"

var osc8OpenClose = regexp.MustCompile(`\x1b\]8;;([^\x07\x1b]*)(?:\x07|\x1b\\)`)

// IsCjkAmbiguousWide reports whether East-Asian ambiguous glyphs should be
// treated as 2 cells (true in CJK locales / terminals).
func IsCjkAmbiguousWide() bool {
	if i18n.IsCjkLanguage() {
		return true
	}
	lang := os.Getenv("LANG") + os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE")
	l := strings.ToLower(lang)
	return strings.Contains(l, "zh") || strings.Contains(l, "ja") || strings.Contains(l, "ko")
}

// StripANSI removes all escape sequences.
func StripANSI(s string) string {
	return ansiGlobal.ReplaceAllString(s, "")
}

// graphemeWidth returns the cell width of a single grapheme cluster.
func graphemeWidth(g string, ambiguousWide bool) int {
	rw := runewidth.Condition{EastAsianWidth: ambiguousWide}
	// Emoji/extended pictographic clusters render as double width.
	w := 0
	for _, r := range g {
		if r == 0x200D || r == 0xFE0F {
			continue
		}
		cw := rw.RuneWidth(r)
		if cw > w {
			w = cw
		}
	}
	return w
}

// VisualLength returns the display width in cells, ignoring ANSI escapes.
func VisualLength(s string) int {
	ambiguousWide := IsCjkAmbiguousWide()
	width := 0
	for _, tok := range splitANSI(s) {
		if tok.ansi {
			continue
		}
		gr := uniseg.NewGraphemes(tok.text)
		for gr.Next() {
			width += graphemeWidth(gr.Str(), ambiguousWide)
		}
	}
	return width
}

type token struct {
	ansi bool
	text string
}

func splitANSI(s string) []token {
	var tokens []token
	i := 0
	for i < len(s) {
		if m := ansiSingle.FindString(s[i:]); m != "" {
			tokens = append(tokens, token{ansi: true, text: m})
			i += len(m)
			continue
		}
		j := i
		for j < len(s) {
			if ansiSingle.MatchString(s[j:]) {
				break
			}
			j++
		}
		tokens = append(tokens, token{ansi: false, text: s[i:j]})
		i = j
	}
	return tokens
}

func sliceVisible(s string, maxVisible int) string {
	if maxVisible <= 0 {
		return ""
	}
	ambiguousWide := IsCjkAmbiguousWide()
	var b strings.Builder
	visible := 0
	i := 0
	for i < len(s) {
		if m := ansiSingle.FindString(s[i:]); m != "" {
			b.WriteString(m)
			i += len(m)
			continue
		}
		j := i
		for j < len(s) {
			if ansiSingle.MatchString(s[j:]) {
				break
			}
			j++
		}
		gr := uniseg.NewGraphemes(s[i:j])
		for gr.Next() {
			g := gr.Str()
			w := graphemeWidth(g, ambiguousWide)
			if visible+w > maxVisible {
				return b.String()
			}
			b.WriteString(g)
			visible += w
		}
		i = j
	}
	return b.String()
}

func closeOpenHyperlink(s string) string {
	matches := osc8OpenClose.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return ""
	}
	last := matches[len(matches)-1]
	if len(last[1]) > 0 {
		return osc8Close
	}
	return ""
}

// TruncateToWidth truncates s to maxWidth cells with an ellipsis suffix.
func TruncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 || VisualLength(s) <= maxWidth {
		return s
	}
	suffix := "..."
	if maxWidth < 3 {
		suffix = strings.Repeat(".", maxWidth)
	}
	keep := maxWidth - len(suffix)
	if keep < 0 {
		keep = 0
	}
	sliced := sliceVisible(s, keep)
	return sliced + closeOpenHyperlink(sliced) + suffix + reset
}

// WrapLineToWidth wraps a line on " | " / " │ " separators to fit maxWidth.
func WrapLineToWidth(line string, maxWidth int) []string {
	if maxWidth <= 0 || VisualLength(line) <= maxWidth {
		return []string{line}
	}
	parts := splitWrapParts(line)
	if len(parts) <= 1 {
		return []string{TruncateToWidth(line, maxWidth)}
	}
	var wrapped []string
	current := parts[0].segment
	for _, p := range parts[1:] {
		candidate := current + p.separator + p.segment
		if VisualLength(candidate) <= maxWidth {
			current = candidate
			continue
		}
		wrapped = append(wrapped, TruncateToWidth(current, maxWidth))
		current = p.segment
	}
	if current != "" {
		wrapped = append(wrapped, TruncateToWidth(current, maxWidth))
	}
	return wrapped
}

type wrapPart struct {
	separator string
	segment   string
}

func splitWrapParts(line string) []wrapPart {
	segments, separators := splitBySeparators(line)
	if len(segments) == 0 {
		return nil
	}
	parts := []wrapPart{{separator: "", segment: segments[0]}}
	for i := 1; i < len(segments); i++ {
		sep := " | "
		if i-1 < len(separators) {
			sep = separators[i-1]
		}
		parts = append(parts, wrapPart{separator: sep, segment: segments[i]})
	}
	// Keep the leading [model] block together.
	firstVisible := strings.TrimLeft(StripANSI(parts[0].segment), " ")
	if strings.HasPrefix(firstVisible, "[") && !strings.Contains(StripANSI(parts[0].segment), "]") && len(parts) > 1 {
		merged := parts[0].segment
		consume := 1
		for consume < len(parts) {
			merged += parts[consume].separator + parts[consume].segment
			consume++
			if strings.Contains(StripANSI(parts[consume-1].segment), "]") {
				break
			}
		}
		parts = append([]wrapPart{{separator: "", segment: merged}}, parts[consume:]...)
	}
	return parts
}

func splitBySeparators(line string) (segments, separators []string) {
	start := 0
	i := 0
	for i < len(line) {
		if m := ansiSingle.FindString(line[i:]); m != "" {
			i += len(m)
			continue
		}
		var sep string
		if strings.HasPrefix(line[i:], " | ") {
			sep = " | "
		} else if strings.HasPrefix(line[i:], " │ ") {
			sep = " │ "
		}
		if sep != "" {
			segments = append(segments, line[start:i])
			separators = append(separators, sep)
			i += len(sep)
			start = i
			continue
		}
		i++
	}
	segments = append(segments, line[start:])
	return
}

// TerminalWidth resolves the terminal width from COLUMNS, then stty, else 0.
func TerminalWidth() int {
	if c := os.Getenv("COLUMNS"); c != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(c)); err == nil && n > 0 {
			return n
		}
	}
	// Best-effort stty size (works when a controlling TTY is present).
	if out, err := exec.Command("stty", "size").Output(); err == nil {
		fields := strings.Fields(string(out))
		if len(fields) == 2 {
			if n, err := strconv.Atoi(fields[1]); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}
