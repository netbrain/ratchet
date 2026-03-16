package views

// blocks maps 8 levels to Unicode block characters.
var blocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders a series of integer values as a Unicode block sparkline.
// Width limits the number of characters (uses the last `width` values).
// Returns empty string for empty input.
func Sparkline(values []int, width int) string {
	if len(values) == 0 || width <= 0 {
		return ""
	}

	// Trim to width (use last N values).
	if len(values) > width {
		values = values[len(values)-width:]
	}

	// Find min/max.
	min, max := values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	span := max - min
	out := make([]rune, len(values))
	maxLevel := len(blocks) - 1
	for i, v := range values {
		if span == 0 {
			out[i] = blocks[len(blocks)/2] // mid-level for flat data
		} else {
			// Use float64 to avoid integer overflow on large value ranges.
			level := int(float64(v-min) * float64(maxLevel) / float64(span))
			// Clamp to valid range in case of floating-point rounding.
			if level < 0 {
				level = 0
			} else if level > maxLevel {
				level = maxLevel
			}
			out[i] = blocks[level]
		}
	}
	return string(out)
}
