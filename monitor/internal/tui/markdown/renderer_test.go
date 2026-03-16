package markdown_test

import (
	"strings"
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/markdown"
)

// ── RenderLines ─────────────────────────────────────────────────────────

func TestRenderLinesH1(t *testing.T) {
	lines := markdown.RenderLines("# Hello World")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Kind != markdown.KindH1 {
		t.Errorf("expected KindH1, got %v", lines[0].Kind)
	}
	if lines[0].Text != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", lines[0].Text)
	}
}

func TestRenderLinesH2(t *testing.T) {
	lines := markdown.RenderLines("## Section Title")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Kind != markdown.KindH2 {
		t.Errorf("expected KindH2, got %v", lines[0].Kind)
	}
	if lines[0].Text != "Section Title" {
		t.Errorf("expected 'Section Title', got %q", lines[0].Text)
	}
}

func TestRenderLinesH3(t *testing.T) {
	lines := markdown.RenderLines("### Subsection")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Kind != markdown.KindH3 {
		t.Errorf("expected KindH3, got %v", lines[0].Kind)
	}
}

func TestHeadingKindsDistinct(t *testing.T) {
	h1 := markdown.RenderLines("# H1")[0]
	h2 := markdown.RenderLines("## H2")[0]
	h3 := markdown.RenderLines("### H3")[0]
	if h1.Kind == h2.Kind || h2.Kind == h3.Kind || h1.Kind == h3.Kind {
		t.Error("each heading level should have a distinct Kind")
	}
}

// ── Bold / inline code ──────────────────────────────────────────────────

func TestRenderLinesBoldStripped(t *testing.T) {
	lines := markdown.RenderLines("This is **bold** text")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0].Text, "bold") {
		t.Errorf("should contain 'bold', got %q", lines[0].Text)
	}
	if strings.Contains(lines[0].Text, "**") {
		t.Errorf("should strip ** markers, got %q", lines[0].Text)
	}
}

func TestRenderLinesInlineCodeStripped(t *testing.T) {
	lines := markdown.RenderLines("Use `go test` to run")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0].Text, "go test") {
		t.Errorf("should contain 'go test', got %q", lines[0].Text)
	}
	if strings.Contains(lines[0].Text, "`") {
		t.Errorf("should strip backtick markers, got %q", lines[0].Text)
	}
}

// ── Code blocks ─────────────────────────────────────────────────────────

func TestRenderLinesCodeBlock(t *testing.T) {
	input := "```go\nfunc main() {\n}\n```"
	lines := markdown.RenderLines(input)
	if len(lines) != 2 {
		t.Fatalf("expected 2 code lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.Kind != markdown.KindCode {
			t.Errorf("expected KindCode, got %v for %q", l.Kind, l.Text)
		}
	}
	if !strings.Contains(lines[0].Text, "func main()") {
		t.Errorf("code block should contain code, got %q", lines[0].Text)
	}
}

func TestRenderLinesCodeBlockNoLang(t *testing.T) {
	input := "```\nsome code\n```"
	lines := markdown.RenderLines(input)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Kind != markdown.KindCode {
		t.Errorf("expected KindCode, got %v", lines[0].Kind)
	}
}

// ── Lists ───────────────────────────────────────────────────────────────

func TestRenderLinesUnorderedList(t *testing.T) {
	input := "- Item one\n- Item two"
	lines := markdown.RenderLines(input)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.Kind != markdown.KindList {
			t.Errorf("expected KindList, got %v", l.Kind)
		}
		if !strings.Contains(l.Text, "•") {
			t.Errorf("list should use bullet char, got %q", l.Text)
		}
	}
}

func TestRenderLinesStarList(t *testing.T) {
	input := "* First\n* Second"
	lines := markdown.RenderLines(input)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Kind != markdown.KindList {
		t.Errorf("expected KindList, got %v", lines[0].Kind)
	}
}

// ── Blockquotes ─────────────────────────────────────────────────────────

func TestRenderLinesBlockquote(t *testing.T) {
	lines := markdown.RenderLines("> This is a quote")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Kind != markdown.KindBlockquote {
		t.Errorf("expected KindBlockquote, got %v", lines[0].Kind)
	}
	if !strings.Contains(lines[0].Text, "This is a quote") {
		t.Errorf("blockquote should contain text, got %q", lines[0].Text)
	}
}

// ── Horizontal rules ────────────────────────────────────────────────────

func TestRenderLinesHorizontalRule(t *testing.T) {
	for _, input := range []string{"---", "***", "___", "-----"} {
		lines := markdown.RenderLines(input)
		if len(lines) != 1 {
			t.Fatalf("expected 1 line for %q, got %d", input, len(lines))
		}
		if lines[0].Kind != markdown.KindRule {
			t.Errorf("expected KindRule for %q, got %v", input, lines[0].Kind)
		}
		if !strings.Contains(lines[0].Text, "─") {
			t.Errorf("horizontal rule should render as line, got %q", lines[0].Text)
		}
	}
}

// ── Plain text ──────────────────────────────────────────────────────────

func TestRenderLinesPlainText(t *testing.T) {
	input := "Just some plain text."
	lines := markdown.RenderLines(input)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Kind != markdown.KindNormal {
		t.Errorf("expected KindNormal, got %v", lines[0].Kind)
	}
	if lines[0].Text != input {
		t.Errorf("plain text should pass through, got %q", lines[0].Text)
	}
}

// ── Empty input ─────────────────────────────────────────────────────────

func TestRenderLinesEmpty(t *testing.T) {
	lines := markdown.RenderLines("")
	if len(lines) != 0 {
		t.Errorf("empty input should produce no lines, got %d", len(lines))
	}
}

// ── Multi-element ───────────────────────────────────────────────────────

func TestRenderLinesMultiElement(t *testing.T) {
	input := "# Title\n\nSome **bold** text.\n\n```\ncode block\n```\n\n- list item"
	lines := markdown.RenderLines(input)

	hasH1 := false
	hasCode := false
	hasList := false
	for _, l := range lines {
		switch l.Kind {
		case markdown.KindH1:
			hasH1 = true
		case markdown.KindCode:
			hasCode = true
		case markdown.KindList:
			hasList = true
		}
	}
	if !hasH1 {
		t.Error("should have H1 line")
	}
	if !hasCode {
		t.Error("should have code line")
	}
	if !hasList {
		t.Error("should have list line")
	}
}

// ── No ANSI codes in output ─────────────────────────────────────────────

func TestRenderLinesNoANSI(t *testing.T) {
	input := "# Title\n**bold** and `code`\n```go\nfunc x() {}\n```\n> quote\n---\n- item"
	lines := markdown.RenderLines(input)
	for _, l := range lines {
		if strings.Contains(l.Text, "\033") {
			t.Errorf("output should not contain ANSI codes, got %q", l.Text)
		}
	}
}

// ── Render (backward compat) ────────────────────────────────────────────

func TestRenderPlainText(t *testing.T) {
	result := markdown.Render("Just some plain text.")
	if result != "Just some plain text." {
		t.Errorf("plain text should pass through, got %q", result)
	}
}

func TestRenderEmpty(t *testing.T) {
	result := markdown.Render("")
	if result != "" {
		t.Errorf("empty input should produce empty output, got %q", result)
	}
}

func TestRenderContainsContent(t *testing.T) {
	input := "# Title\n\nSome text.\n\n- list item"
	result := markdown.Render(input)
	if !strings.Contains(result, "Title") {
		t.Error("should contain title")
	}
	if !strings.Contains(result, "list item") {
		t.Error("should contain list item")
	}
}
