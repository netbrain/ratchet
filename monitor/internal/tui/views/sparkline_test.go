package views_test

import (
	"strings"
	"testing"

	"github.com/netbrain/ratchet-monitor/internal/tui/views"
)

func TestSparklineBasic(t *testing.T) {
	result := views.Sparkline([]int{1, 3, 5, 7, 2, 4, 6, 8}, 8)
	if len(result) == 0 {
		t.Fatal("Sparkline returned empty string")
	}
	// Should contain Unicode block characters
	for _, r := range result {
		if r < '▁' || r > '█' {
			t.Errorf("unexpected char %q in sparkline", string(r))
		}
	}
}

func TestSparklineWidth(t *testing.T) {
	values := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := views.Sparkline(values, 5)
	// Output should be at most 5 runes wide (may use last N values)
	runes := []rune(result)
	if len(runes) > 5 {
		t.Errorf("Sparkline width = %d runes, want <= 5", len(runes))
	}
}

func TestSparklineEmpty(t *testing.T) {
	result := views.Sparkline(nil, 8)
	if result != "" {
		t.Errorf("Sparkline(nil) = %q, want empty", result)
	}
	result = views.Sparkline([]int{}, 8)
	if result != "" {
		t.Errorf("Sparkline([]) = %q, want empty", result)
	}
}

func TestSparklineSingleValue(t *testing.T) {
	result := views.Sparkline([]int{5}, 8)
	runes := []rune(result)
	if len(runes) != 1 {
		t.Errorf("single value sparkline should be 1 char, got %d", len(runes))
	}
}

func TestSparklineAllSameValues(t *testing.T) {
	result := views.Sparkline([]int{3, 3, 3, 3}, 4)
	// Should not panic (no divide by zero) and produce valid output
	if len(result) == 0 {
		t.Fatal("all-same-values sparkline should produce output")
	}
	// All chars should be the same
	runes := []rune(result)
	for _, r := range runes {
		if r != runes[0] {
			t.Errorf("all-same-values should produce uniform chars, got mixed")
			break
		}
	}
}

func TestSparklineMinMax(t *testing.T) {
	// Min value should use lowest block, max should use highest
	result := views.Sparkline([]int{0, 100}, 2)
	runes := []rune(result)
	if len(runes) != 2 {
		t.Fatalf("expected 2 chars, got %d", len(runes))
	}
	if runes[0] >= runes[1] {
		t.Errorf("min value char (%q) should be lower than max (%q)", string(runes[0]), string(runes[1]))
	}
}

func TestSparklineContainsOnlyBlockChars(t *testing.T) {
	result := views.Sparkline([]int{1, 4, 2, 8, 3, 7, 5, 6}, 8)
	blocks := "▁▂▃▄▅▆▇█"
	for _, r := range result {
		if !strings.ContainsRune(blocks, r) {
			t.Errorf("unexpected char %q, expected Unicode block char", string(r))
		}
	}
}

// ── Edge cases: overflow and extremes ───────────────────────────────────

func TestSparklineNegativeWidth(t *testing.T) {
	result := views.Sparkline([]int{1, 2, 3}, -1)
	if result != "" {
		t.Errorf("Sparkline with negative width = %q, want empty", result)
	}
}

func TestSparklineZeroWidth(t *testing.T) {
	result := views.Sparkline([]int{1, 2, 3}, 0)
	if result != "" {
		t.Errorf("Sparkline with zero width = %q, want empty", result)
	}
}

func TestSparklineNegativeValues(t *testing.T) {
	result := views.Sparkline([]int{-10, -5, 0, 5, 10}, 5)
	runes := []rune(result)
	if len(runes) != 5 {
		t.Fatalf("expected 5 chars, got %d", len(runes))
	}
	// First char (min=-10) should be lowest block, last char (max=10) should be highest
	if runes[0] >= runes[4] {
		t.Errorf("min char (%q) should be lower than max (%q)", string(runes[0]), string(runes[4]))
	}
}

func TestSparklineLargeIntegerRange(t *testing.T) {
	// Values that would overflow int multiplication in 64-bit: max-min = large.
	// (v-min) * 7 could overflow int if using pure integer arithmetic.
	const big = 1<<50
	result := views.Sparkline([]int{-big, 0, big}, 3)
	runes := []rune(result)
	if len(runes) != 3 {
		t.Fatalf("expected 3 chars, got %d", len(runes))
	}
	blocks := "▁▂▃▄▅▆▇█"
	for _, r := range runes {
		if !strings.ContainsRune(blocks, r) {
			t.Errorf("unexpected char %q in large-range sparkline", string(r))
		}
	}
	// min should map to lowest, max to highest
	if runes[0] != '▁' {
		t.Errorf("min value should be lowest block, got %q", string(runes[0]))
	}
	if runes[2] != '█' {
		t.Errorf("max value should be highest block, got %q", string(runes[2]))
	}
}

func TestSparklineWidthOne(t *testing.T) {
	// Width=1 with many values should use the last value only.
	result := views.Sparkline([]int{1, 2, 3, 4, 5}, 1)
	runes := []rune(result)
	if len(runes) != 1 {
		t.Errorf("width=1 sparkline should be 1 char, got %d", len(runes))
	}
}

func TestSparklineTwoIdenticalValues(t *testing.T) {
	result := views.Sparkline([]int{42, 42}, 10)
	runes := []rune(result)
	if len(runes) != 2 {
		t.Fatalf("expected 2 chars, got %d", len(runes))
	}
	if runes[0] != runes[1] {
		t.Errorf("identical values should produce identical chars")
	}
}
