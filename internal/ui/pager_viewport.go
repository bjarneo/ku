package ui

import (
	"slices"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// pagerViewport is the scrollable view under the pager. It mirrors the soft
// wrap, horizontal scroll and offset behavior of bubbles' viewport, but keeps a
// per-line width and a prefix sum of wrapped row heights. Offset math and
// rendering then cost the visible rows instead of the whole buffer, which is
// what made the log pager slower as the buffer filled: bubbles measures every
// line on each frame.
type pagerViewport struct {
	width  int
	height int

	// SoftWrap wraps long lines onto multiple rows. When false, one logical
	// line is one row and the x offset scrolls horizontally.
	SoftWrap bool

	yOffset int
	xOffset int

	horizontalStep int
	keys           pagerViewportKeys

	lines      []string
	lineWidths []int // screen width per line, cached; recomputed only on SetContentLines and AppendLines
	prefix     []int // prefix[i] = wrapped rows of lines[:i]; always len(lines)+1
	longest    int   // widest line in cells, for horizontal scrolling
}

// pagerViewportKeys matches bubbles' default viewport bindings so the keys that
// scroll the logs stay the same as in every other pager-like TUI.
type pagerViewportKeys struct {
	pageDown     key.Binding
	pageUp       key.Binding
	halfPageUp   key.Binding
	halfPageDown key.Binding
	down         key.Binding
	up           key.Binding
	left         key.Binding
	right        key.Binding
}

func defaultPagerViewportKeys() pagerViewportKeys {
	return pagerViewportKeys{
		pageDown:     key.NewBinding(key.WithKeys("pgdown", "space", "f")),
		pageUp:       key.NewBinding(key.WithKeys("pgup", "b")),
		halfPageUp:   key.NewBinding(key.WithKeys("u", "ctrl+u")),
		halfPageDown: key.NewBinding(key.WithKeys("d", "ctrl+d")),
		down:         key.NewBinding(key.WithKeys("down", "j")),
		up:           key.NewBinding(key.WithKeys("up", "k")),
		left:         key.NewBinding(key.WithKeys("left", "h")),
		right:        key.NewBinding(key.WithKeys("right", "l")),
	}
}

func newPagerViewport() pagerViewport {
	return pagerViewport{horizontalStep: 6, keys: defaultPagerViewportKeys()}
}

// contentWidth is the width available to a line. The pager draws no frame,
// gutter or margin of its own.
func (m pagerViewport) contentWidth() int { return m.width }

// wrappedRows is the number of rows a line of cell width w takes, matching
// bubbles' viewport math: at least one row, and one row per width slice.
func wrappedRows(w, maxWidth int) int {
	if maxWidth < 1 {
		maxWidth = 1
	}
	if w <= 0 {
		return 1
	}
	return (w + maxWidth - 1) / maxWidth
}

// rebuildPrefix recomputes the wrapped-row prefix sums from the cached line
// widths. It runs on resize, on a full content replace, and when the pager
// crosses a trim boundary, never on the streaming hot path.
func (m *pagerViewport) rebuildPrefix() {
	if cap(m.prefix) < len(m.lines)+1 {
		m.prefix = make([]int, len(m.lines)+1)
	} else {
		m.prefix = m.prefix[:len(m.lines)+1]
	}
	base := 0
	for i, w := range m.lineWidths {
		base += wrappedRows(w, m.contentWidth())
		m.prefix[i+1] = base
	}
}

// ensureIndex restores the prefix invariant after a caller changed the line
// slices without going through SetContentLines or AppendLines.
func (m *pagerViewport) ensureIndex() {
	if len(m.prefix) != len(m.lines)+1 {
		m.rebuildPrefix()
	}
}

// SetContentLines replaces the whole buffer. It measures every line, so callers
// use AppendLines when the change is an append.
func (m *pagerViewport) SetContentLines(lines []string) {
	if len(lines) == 1 && ansi.StringWidth(lines[0]) == 0 {
		lines = nil // a lone empty line is no content, matching bubbles
	}
	m.lines = lines
	if cap(m.lineWidths) < len(lines) {
		m.lineWidths = make([]int, len(lines))
	} else {
		m.lineWidths = m.lineWidths[:len(lines)]
	}
	m.longest = 0
	for i, s := range lines {
		w := ansi.StringWidth(s)
		m.lineWidths[i] = w
		if w > m.longest {
			m.longest = w
		}
	}
	m.rebuildPrefix()
	if m.yOffset > m.maxYOffset() {
		m.GotoBottom()
	}
}

// AppendLines adds lines to the end and measures only the new ones.
func (m *pagerViewport) AppendLines(lines []string) {
	if len(lines) == 0 {
		return
	}
	m.ensureIndex()
	w := m.contentWidth()
	for _, s := range lines {
		lw := ansi.StringWidth(s)
		m.lines = append(m.lines, s)
		m.lineWidths = append(m.lineWidths, lw)
		if lw > m.longest {
			m.longest = lw
		}
		m.prefix = append(m.prefix, m.prefix[len(m.prefix)-1]+wrappedRows(lw, w))
	}
}

// lineAtRow converts a row offset into a total row count, the logical line at
// that offset, and the row offset inside that line. bubbles calls this
// calculateLine; the prefix sum makes it O(log n) with soft wrap on and O(1)
// with it off.
func (m *pagerViewport) lineAtRow(yoffset int) (total, ridx, voffset int) {
	if len(m.lines) == 0 {
		return 0, 0, 0
	}
	if !m.SoftWrap {
		return len(m.lines), min(yoffset, len(m.lines)), 0
	}
	m.ensureIndex()
	total = m.prefix[len(m.lines)]
	if yoffset >= total {
		return total, len(m.lines), 0
	}
	i := sort.Search(len(m.lines), func(i int) bool { return m.prefix[i+1] > yoffset })
	return total, i, yoffset - m.prefix[i]
}

// View renders the visible rows, padded to the viewport size.
func (m *pagerViewport) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	content := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(strings.Join(m.visibleLines(), "\n"))
	return content
}

// visibleLines returns the rows that should currently be shown. With soft wrap
// on, only the lines that can reach the view are measured.
func (m *pagerViewport) visibleLines() []string {
	if m.width == 0 || m.height == 0 {
		return nil
	}
	total, ridx, voffset := m.lineAtRow(m.yOffset)
	if total == 0 {
		return nil
	}
	maxWidth := m.contentWidth()
	bottom := clamp(ridx+m.height, ridx, len(m.lines))
	lines := slices.Clone(m.lines[ridx:bottom])
	if m.SoftWrap {
		return m.softWrap(lines, maxWidth, m.height, voffset)
	}
	if m.xOffset == 0 && m.longest <= maxWidth {
		return lines
	}
	for i := range lines {
		lines[i] = ansi.Cut(lines[i], m.xOffset, m.xOffset+maxWidth)
	}
	return lines
}

// softWrap expands the rows of the given lines and returns the window starting
// at voffset. Same cut windows as bubbles' viewport.
func (m pagerViewport) softWrap(lines []string, maxWidth, maxHeight, voffset int) []string {
	wrapped := make([]string, 0, maxHeight)
	for _, line := range lines {
		lineWidth := ansi.StringWidth(line)
		if lineWidth <= maxWidth {
			wrapped = append(wrapped, line)
			continue
		}
		for idx := 0; idx < lineWidth; idx += maxWidth {
			wrapped = append(wrapped, ansi.Cut(line, idx, maxWidth+idx))
		}
	}
	if voffset >= len(wrapped) {
		return nil
	}
	return wrapped[voffset:min(voffset+maxHeight, len(wrapped))]
}

// maxYOffset is the largest row offset that still shows content.
func (m *pagerViewport) maxYOffset() int {
	total, _, _ := m.lineAtRow(0)
	return max(0, total-m.height)
}

// maxXOffset is the furthest horizontal scroll with soft wrap off.
func (m pagerViewport) maxXOffset() int {
	return max(0, m.longest-m.contentWidth())
}

func (m pagerViewport) Height() int  { return m.height }
func (m pagerViewport) Width() int   { return m.width }
func (m pagerViewport) YOffset() int { return m.yOffset }
func (m pagerViewport) XOffset() int { return m.xOffset }

func (m *pagerViewport) SetHeight(h int) { m.height = h }

// SetWidth resizes the viewport and reflows the wrapped-row index from the
// cached widths. It is O(lines) once per resize, not per line.
func (m *pagerViewport) SetWidth(w int) {
	if w == m.width {
		return
	}
	m.width = w
	m.rebuildPrefix()
}

// SetYOffset scrolls vertically, clamped to the content.
func (m *pagerViewport) SetYOffset(n int) {
	m.yOffset = clamp(n, 0, m.maxYOffset())
}

// SetXOffset scrolls horizontally. It is a no-op with soft wrap on.
func (m *pagerViewport) SetXOffset(n int) {
	if m.SoftWrap {
		return
	}
	m.xOffset = clamp(n, 0, m.maxXOffset())
}

func (m pagerViewport) AtTop() bool    { return m.yOffset <= 0 }
func (m pagerViewport) AtBottom() bool { return m.yOffset >= m.maxYOffset() }

// GotoTop scrolls to the first row.
func (m *pagerViewport) GotoTop() {
	m.yOffset = 0
}

// GotoBottom scrolls to the last row.
func (m *pagerViewport) GotoBottom() {
	m.yOffset = m.maxYOffset()
}

// ScrollPercent is the fraction of the content scrolled, for the pane header.
func (m *pagerViewport) ScrollPercent() float64 {
	total, _, _ := m.lineAtRow(0)
	if m.height >= total {
		return 1.0
	}
	v := float64(m.yOffset) / float64(total-m.height)
	return min(1, max(0, v))
}

func (m *pagerViewport) ScrollDown(n int) {
	if m.AtBottom() || n == 0 || len(m.lines) == 0 {
		return
	}
	m.SetYOffset(m.yOffset + n)
}

func (m *pagerViewport) ScrollUp(n int) {
	if m.AtTop() || n == 0 || len(m.lines) == 0 {
		return
	}
	m.SetYOffset(m.yOffset - n)
}

func (m *pagerViewport) PageDown() {
	if m.AtBottom() {
		return
	}
	m.ScrollDown(m.height)
}

func (m *pagerViewport) PageUp() {
	if m.AtTop() {
		return
	}
	m.ScrollUp(m.height)
}

func (m *pagerViewport) HalfPageDown() {
	if m.AtBottom() {
		return
	}
	m.ScrollDown(m.height / 2) //nolint:mnd
}

func (m *pagerViewport) HalfPageUp() {
	if m.AtTop() {
		return
	}
	m.ScrollUp(m.height / 2) //nolint:mnd
}

func (m *pagerViewport) ScrollLeft(n int)  { m.SetXOffset(m.xOffset - n) }
func (m *pagerViewport) ScrollRight(n int) { m.SetXOffset(m.xOffset + n) }

// Update applies the default viewport scroll keys. Mouse input never reaches
// the pager: ku only enables mouse reporting for the embedded terminal.
func (m *pagerViewport) Update(msg tea.Msg) (pagerViewport, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}
	switch {
	case key.Matches(km, m.keys.pageDown):
		m.PageDown()
	case key.Matches(km, m.keys.pageUp):
		m.PageUp()
	case key.Matches(km, m.keys.halfPageDown):
		m.HalfPageDown()
	case key.Matches(km, m.keys.halfPageUp):
		m.HalfPageUp()
	case key.Matches(km, m.keys.down):
		m.ScrollDown(1)
	case key.Matches(km, m.keys.up):
		m.ScrollUp(1)
	case key.Matches(km, m.keys.left):
		m.ScrollLeft(m.horizontalStep)
	case key.Matches(km, m.keys.right):
		m.ScrollRight(m.horizontalStep)
	}
	return *m, nil
}
