// Package markdown provides terminal-native markdown rendering for go-tui.
//
// Since go-tui uses a cell-based renderer, raw ANSI escape codes in text
// strings are rendered as literal characters. Instead, this package returns
// structured StyledLine data that callers convert to tui.WithTextStyle().
package markdown

import (
	"strings"
)

// LineKind identifies the markdown element type of a rendered line.
type LineKind int

const (
	KindNormal     LineKind = iota
	KindH1                  // # heading
	KindH2                  // ## heading
	KindH3                  // ### heading
	KindCode                // inside ``` block
	KindBlockquote          // > quote
	KindRule                // --- / *** / ___
	KindList                // - item / * item
)

// StyledLine represents a single rendered line with its style hint.
type StyledLine struct {
	Text string
	Kind LineKind
}

// RenderLines converts markdown text to a slice of styled lines.
// Each line carries a Kind that callers use to apply appropriate styling.
func RenderLines(input string) []StyledLine {
	if input == "" {
		return nil
	}

	input = sanitize(input)
	lines := strings.Split(input, "\n")
	var out []StyledLine
	inCodeBlock := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Code block toggle.
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				inCodeBlock = true
				continue
			}
			inCodeBlock = false
			continue
		}

		if inCodeBlock {
			out = append(out, StyledLine{Text: "  " + line, Kind: KindCode})
			continue
		}

		// Headers.
		if strings.HasPrefix(line, "### ") {
			text := strings.TrimPrefix(line, "### ")
			out = append(out, StyledLine{Text: text, Kind: KindH3})
			continue
		}
		if strings.HasPrefix(line, "## ") {
			text := strings.TrimPrefix(line, "## ")
			out = append(out, StyledLine{Text: text, Kind: KindH2})
			continue
		}
		if strings.HasPrefix(line, "# ") {
			text := strings.TrimPrefix(line, "# ")
			out = append(out, StyledLine{Text: text, Kind: KindH1})
			continue
		}

		// Horizontal rule.
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= 3 && (allSameChar(trimmed, '-') || allSameChar(trimmed, '*') || allSameChar(trimmed, '_')) {
			out = append(out, StyledLine{Text: strings.Repeat("─", 40), Kind: KindRule})
			continue
		}

		// Blockquote.
		if strings.HasPrefix(line, "> ") {
			text := strings.TrimPrefix(line, "> ")
			out = append(out, StyledLine{Text: "│ " + text, Kind: KindBlockquote})
			continue
		}

		// Unordered list.
		if strings.HasPrefix(line, "- ") {
			text := strings.TrimPrefix(line, "- ")
			out = append(out, StyledLine{Text: "  • " + stripInlineMarkers(text), Kind: KindList})
			continue
		}
		if strings.HasPrefix(line, "* ") {
			text := strings.TrimPrefix(line, "* ")
			out = append(out, StyledLine{Text: "  • " + stripInlineMarkers(text), Kind: KindList})
			continue
		}

		// Regular line — strip inline markdown markers for clean display.
		out = append(out, StyledLine{Text: stripInlineMarkers(line), Kind: KindNormal})
	}

	return out
}

// Render converts markdown text to plain text with structural formatting
// (bullets, indentation, horizontal rules) but no ANSI codes.
// Kept for backward compatibility; prefer RenderLines for styled output.
func Render(input string) string {
	lines := RenderLines(input)
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, l := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(l.Text)
	}
	return sb.String()
}

// stripInlineMarkers removes **bold** and `code` markers, keeping the text.
func stripInlineMarkers(line string) string {
	var out strings.Builder
	i := 0
	for i < len(line) {
		// Bold: **text**
		if i+1 < len(line) && line[i] == '*' && line[i+1] == '*' {
			end := strings.Index(line[i+2:], "**")
			if end >= 0 {
				out.WriteString(line[i+2 : i+2+end])
				i = i + 2 + end + 2
				continue
			}
		}
		// Inline code: `text`
		if line[i] == '`' {
			end := strings.Index(line[i+1:], "`")
			if end >= 0 {
				out.WriteString(line[i+1 : i+1+end])
				i = i + 1 + end + 1
				continue
			}
		}
		out.WriteByte(line[i])
		i++
	}
	return out.String()
}

// sanitize strips raw ANSI escape sequences from input.
func sanitize(s string) string {
	return strings.ReplaceAll(s, "\x1b", "")
}

func allSameChar(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != c {
			return false
		}
	}
	return len(s) > 0
}
