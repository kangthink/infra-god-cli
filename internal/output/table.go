package output

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
)

var (
	Green   = color.New(color.FgGreen).SprintFunc()
	Yellow  = color.New(color.FgYellow).SprintFunc()
	Red     = color.New(color.FgRed).SprintFunc()
	Cyan    = color.New(color.FgCyan).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
	Dim     = color.New(color.Faint).SprintFunc()
	BoldRed = color.New(color.Bold, color.FgRed).SprintFunc()
)

// Table renders aligned columnar output.
type Table struct {
	Headers []string
	Rows    [][]string
	MinWidth int
}

// NewTable creates a table with headers.
func NewTable(headers ...string) *Table {
	return &Table{
		Headers:  headers,
		MinWidth: 2,
	}
}

// AddRow appends a row.
func (t *Table) AddRow(cols ...string) {
	t.Rows = append(t.Rows, cols)
}

// Render produces the formatted table string.
func (t *Table) Render() string {
	// Calculate column widths
	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = visibleLen(h) + t.MinWidth
	}
	for _, row := range t.Rows {
		for i, col := range row {
			if i < len(widths) {
				w := visibleLen(col) + t.MinWidth
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	var sb strings.Builder

	// Header
	for i, h := range t.Headers {
		sb.WriteString(Bold(padRight(h, widths[i])))
	}
	sb.WriteString("\n")

	// Rows
	for _, row := range t.Rows {
		for i, col := range row {
			if i < len(widths) {
				sb.WriteString(padRight(col, widths[i]))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// StatusIcon returns a colored fixed-width status icon (2 chars).
func StatusIcon(status string) string {
	switch status {
	case "ok":
		return Green("◆")
	case "warning":
		return Yellow("▲")
	case "critical":
		return Red("✗")
	case "error":
		return Red("✗")
	case "stopped":
		return Dim("○")
	case "auth_fail":
		return Yellow("▲")
	default:
		return "?"
	}
}

// ColorPercent returns a colored percentage string based on thresholds.
func ColorPercent(val int, warn int, crit int) string {
	s := fmt.Sprintf("%d%%", val)
	if val >= crit {
		return Red(s)
	}
	if val >= warn {
		return Yellow(s)
	}
	return Green(s)
}

// ColorLoad returns a colored load average based on CPU count.
func ColorLoad(load float64, cpus int) string {
	s := fmt.Sprintf("%.1f", load)
	threshold := float64(cpus) * 0.8
	if load >= threshold {
		return Red(s)
	}
	if load >= threshold*0.7 {
		return Yellow(s)
	}
	return s
}

// Header prints a formatted section header.
func Header(title string) string {
	return fmt.Sprintf("\n %s\n %s", Bold(title), strings.Repeat("─", 65))
}

// SubHeader prints a sub-section header.
func SubHeader(title string) string {
	return fmt.Sprintf("\n %s %s", Cyan(title), strings.Repeat("─", 55-len(title)))
}

func padRight(s string, width int) string {
	vl := visibleLen(s)
	if vl >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-vl)
}

// visibleLen returns the visible terminal width of a string, stripping ANSI codes
// and accounting for wide characters (emoji, CJK).
func visibleLen(s string) int {
	stripped := StripANSI(s)
	// runewidth doesn't handle variation selector (U+FE0F) correctly.
	// Characters followed by FE0F display as 2-wide emoji in terminals.
	runes := []rune(stripped)
	width := 0
	for i := 0; i < len(runes); i++ {
		if runes[i] == 0xFE0F {
			// variation selector itself is zero-width, but the preceding
			// character becomes 2-wide. If we already counted it as 1, add 1.
			if width > 0 {
				prev := runes[i-1]
				if runewidth.RuneWidth(prev) < 2 {
					width++
				}
			}
			continue
		}
		width += runewidth.RuneWidth(runes[i])
	}
	return width
}

// ansiExtraLen returns the difference between byte length and visible length.
func ansiExtraLen(s string) int {
	return len(s) - visibleLen(s)
}

// StripANSI removes ANSI escape codes from a string.
func StripANSI(s string) string {
	var sb strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
