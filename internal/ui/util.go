package ui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func itoa(n int) string { return strconv.Itoa(n) }

const tabWidth = 8

// expandTabs replaces tab characters with spaces up to the next 8-column tab
// stop. Terminals draw a tab as a jump to the next stop, but width measuring
// (ansi.StringWidth) counts it as zero, so an untreated tab makes a line measure
// narrower than it renders and spill past its pane. Existing escape sequences
// are preserved; only the column count is approximate for them, which at worst
// shifts a tab stop but never causes overflow.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + tabWidth)
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

// newFilterInput builds the "/"-prompted text input shared by the table and
// logs filters: same prompt and styling, only the placeholder differs.
func newFilterInput(placeholder string) textinput.Model {
	fi := textinput.New()
	fi.Prompt = "/"
	fi.Placeholder = placeholder
	styles := fi.Styles()
	styles.Cursor.Blink = false
	fi.SetStyles(styles)
	return fi
}

// truncate shortens s to at most w display columns, adding an ellipsis when
// cut. It is display-width and ANSI aware (the same engine used everywhere else
// in the package).
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// spread lays out left and right text on one line of the given width, padding
// the gap between them (minimum one space).
func spread(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	rw := lipgloss.Width(right)
	if rw >= width {
		return ansi.Truncate(right, width, "")
	}
	if lipgloss.Width(left)+1+rw > width {
		budget := width - rw - 1
		if budget <= 0 {
			left = ""
		} else {
			left = ansi.Truncate(left, budget, "…")
		}
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

const (
	// paneGap separates the sidebar from the active content pane.
	paneGap      = 1
	panePaddingY = 0
	panePaddingX = 1
)

// paneStyleWidth/paneStyleHeight are the values to pass to a bordered pane
// style's Width/Height. In lipgloss v2 those set the box's total size (border
// and padding included), so they are simply the outer size; the content area
// then works out to paneContentWidth/paneContentHeight. Returning outer-2 here
// (a pre-v2 habit) shrank the box and squeezed the content area, which wrapped
// full-width lines and pushed the bottom border out of view.
func paneStyleWidth(outer int) int {
	return outer
}

func paneStyleHeight(outer int) int {
	return outer
}

func paneContentWidth(outer int) int {
	return paneInnerSize(outer, 2+2*panePaddingX)
}

func paneContentHeight(outer int) int {
	return paneInnerSize(outer, 2+2*panePaddingY)
}

func paneInnerSize(outer, frame int) int {
	if n := outer - frame; n > 0 {
		return n
	}
	return 1
}
