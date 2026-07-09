package ui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// maxLogLines caps the streamed buffer so an endless log can't grow without
// bound. It only applies to the append path (storeLine); content set in one shot
// (SetContent) is not trimmed.
const maxLogLines = 5000

// pager is a scrollable text viewport with regex filtering, wrap toggling, and
// whole-line selection/copy. It backs every read-only text screen (logs, YAML
// detail, config summary); each embeds a pager and only supplies its content and
// screen-specific keys. Content is held as a []string and pushed with
// SetContentLines, so appends are amortized O(1) and there is no giant joined
// string to re-split on every sync.
type pager struct {
	th     Theme
	vp     viewport.Model
	title  string
	follow bool
	height int // pane content height, retained so chrome changes can relayout

	lines    []string // raw buffer
	filtered []string // lines currently shown (all of them when no filter is active)

	// Filtering. The filter is a regular expression matched against each line;
	// an empty filter shows everything and an invalid pattern shows everything
	// (re stays nil) so results update as the user types.
	filtering bool
	filter    textinput.Model
	re        *regexp.Regexp
	matched   int // lines currently shown

	// Visual line selection: v enters selection mode with a movable cursor; m drops
	// the anchor (marking) and further movement extends the range to copy. While
	// selecting, the view is frozen on a snapshot (selLines). The wrap mode is left
	// untouched so entering selection never reflows or scrolls the view; rows map
	// to lines with soft-wrap-aware math.
	selecting bool
	marking   bool
	selAnchor int
	selCursor int
	selLines  []string
}

func newPager(th Theme) pager {
	vp := viewport.New()
	vp.SoftWrap = true // wrap long lines so the full line is visible, not truncated
	return pager{th: th, vp: vp, follow: true, filter: newFilterInput("filter (regex)")}
}

func (p *pager) setSize(w, h int) {
	if h < 1 {
		h = 1
	}
	p.height = h
	p.vp.SetWidth(w)
	if fw := w - 8; fw > 4 {
		p.filter.SetWidth(fw) // bound the filter input so it can't overflow
	}
	p.relayout()
}

// chromeLines counts the non-viewport rows the pane draws: the title, plus the
// filter row when filtering or a filter is applied.
func (p *pager) chromeLines() int {
	if p.filtering || p.filterActive() {
		return 2
	}
	return 1
}

// relayout sizes the viewport to the height left after the chrome, keeping the
// tail in view when following.
func (p *pager) relayout() {
	h := p.height - p.chromeLines()
	if h < 1 {
		h = 1
	}
	p.vp.SetHeight(h)
	p.stickToBottom()
}

// stickToBottom keeps the newest lines in view while following.
func (p *pager) stickToBottom() {
	if p.follow {
		p.vp.GotoBottom()
	}
}

// SetContent replaces the buffer with a single string (split on newlines) and
// scrolls to the top. For one-shot content such as YAML or a config summary.
func (p *pager) SetContent(s string) { p.SetLines(strings.Split(s, "\n")) }

// SetLines replaces the buffer with lines and scrolls to the top, applying any
// active filter.
func (p *pager) SetLines(lines []string) {
	p.clearSelection()
	p.lines = lines
	p.rebuildContent()
	p.vp.SetContentLines(p.filtered)
	p.vp.GotoTop()
}

func (p *pager) appendLine(s string) {
	p.storeLine(s)
	p.syncViewport()
}

// storeLine adds a line to the buffer and the filtered view without touching
// the viewport, so a burst of lines can be stored and then synced once.
func (p *pager) storeLine(s string) {
	s = expandTabs(s) // tabs measure as zero width and would spill past the pane
	p.lines = append(p.lines, s)
	if len(p.lines) > maxLogLines {
		p.lines = p.lines[len(p.lines)-maxLogLines:]
		p.rebuildContent() // rebuild only when trimming the front
		return
	}
	if p.re != nil && !p.re.MatchString(ansi.Strip(s)) {
		return // filtered out: nothing changed on screen
	}
	p.filtered = append(p.filtered, s)
	p.matched = len(p.filtered)
}

// syncViewport pushes the current content into the viewport, sticking to the
// tail while following. It is a no-op while selecting, so the view stays frozen
// on the selection snapshot as new lines keep arriving.
func (p *pager) syncViewport() {
	if p.selecting {
		return
	}
	p.vp.SetContentLines(p.filtered)
	p.stickToBottom()
}

// --- visual selection -------------------------------------------------------

// wrappedHeight is how many rows a line occupies at width w, matching the
// viewport's own soft-wrap math (see viewport.calculateLine).
func wrappedHeight(s string, w int) int {
	if w < 1 {
		w = 1
	}
	return max(1, (ansi.StringWidth(s)+w-1)/w)
}

// visibleWidth is the wrapping width the viewport uses for a line.
func (p *pager) visibleWidth() int {
	if w := p.vp.Width(); w > 0 {
		return w
	}
	return 1
}

// viewLines is the slice currently shown in the viewport: the frozen snapshot
// while selecting, otherwise the live filtered lines.
func (p *pager) viewLines() []string {
	if p.selecting {
		return p.selLines
	}
	return p.filtered
}

// lineAtRow maps a viewport screen row (0-based from the top of the visible
// area) to a logical line index in the current view. It honors the soft-wrap
// state, so it reads the view as displayed and never has to reflow it.
func (p *pager) lineAtRow(row int) int {
	lines := p.viewLines()
	n := len(lines)
	if n == 0 {
		return 0
	}
	off := p.vp.YOffset()
	if !p.vp.SoftWrap {
		return clamp(off+row, 0, n-1)
	}
	target := off + row
	acc, w := 0, p.visibleWidth()
	for i, ln := range lines {
		acc += wrappedHeight(ln, w)
		if target < acc {
			return i
		}
	}
	return n - 1
}

// wrappedRowOf returns the wrapped-row offset of a logical line's first row in
// the current view, i.e. the YOffset that puts that line at the top.
func (p *pager) wrappedRowOf(line int) int {
	if !p.vp.SoftWrap {
		return line
	}
	lines := p.viewLines()
	acc, w := 0, p.visibleWidth()
	for i := 0; i < line && i < len(lines); i++ {
		acc += wrappedHeight(lines[i], w)
	}
	return acc
}

// beginSelect freezes the view on a snapshot. The wrap mode and scroll offset
// are left as they are, so entering selection never reflows or moves the view.
func (p *pager) beginSelect() {
	// Clone the view so lines streaming in while selecting (which keep appending
	// to p.filtered) can't mutate the frozen snapshot.
	p.selLines = slices.Clone(p.filtered)
	p.selecting = true
	p.follow = false
}

// startSelect enters visual line selection, anchored at the top visible line.
func (p *pager) startSelect() {
	if len(p.filtered) == 0 {
		return
	}
	p.beginSelect()
	p.marking = false
	p.selAnchor = p.lineAtRow(0)
	p.selCursor = p.selAnchor
	p.renderSelection()
}

// mark drops the selection anchor at the cursor; further movement extends the
// marked range from here.
func (p *pager) mark() {
	p.marking = true
	p.selAnchor = p.selCursor
	p.renderSelection()
}

// stopSelect leaves selection and restores the live view.
func (p *pager) stopSelect() {
	p.clearSelection()
	p.syncViewport()
}

func (p *pager) clearSelection() {
	p.selecting = false
	p.marking = false
	p.selLines = nil
}

func (p *pager) moveSel(d int) { p.setSelCursor(p.selCursor + d) }
func (p *pager) setSelCursor(i int) {
	p.selCursor = clamp(i, 0, len(p.selLines)-1)
	p.renderSelection()
	p.keepCursorVisible()
}

// keepCursorVisible scrolls the viewport just enough to show the cursor line,
// used while moving/dragging. It is not called on selection start, so pressing
// to begin a selection never shifts the view.
func (p *pager) keepCursorVisible() {
	row := p.wrappedRowOf(p.selCursor)
	off, h := p.vp.YOffset(), p.vp.Height()
	switch {
	case row < off:
		p.vp.SetYOffset(row)
	case row >= off+h:
		p.vp.SetYOffset(row - h + 1)
	}
}

// selRange is the inclusive [lo, hi] line range to copy: just the cursor line
// until the anchor is marked, then the span between anchor and cursor.
func (p *pager) selRange() (int, int) {
	if !p.marking {
		return p.selCursor, p.selCursor
	}
	if p.selAnchor <= p.selCursor {
		return p.selAnchor, p.selCursor
	}
	return p.selCursor, p.selAnchor
}

func (p *pager) selCount() int { lo, hi := p.selRange(); return hi - lo + 1 }

// renderSelection redraws the frozen snapshot with the marked range highlighted.
// It does not touch the scroll offset, so the view only moves when movement calls
// keepCursorVisible. Selected lines stay full-length so the viewport owns all
// wrapping and horizontal scrolling.
func (p *pager) renderSelection() {
	lo, hi := p.selRange()
	rows := make([]string, len(p.selLines))
	for i, ln := range p.selLines {
		if i < lo || i > hi {
			rows[i] = ln
			continue
		}
		rows[i] = p.th.SelItemSel.Render(ln) // no fixed Width: avoids injected newlines
	}
	p.vp.SetContentLines(rows)
}

// copySelection returns the marked lines as plain text (ANSI stripped).
func (p *pager) copySelection() string {
	lo, hi := p.selRange()
	rows := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi && i < len(p.selLines); i++ {
		rows = append(rows, ansi.Strip(p.selLines[i]))
	}
	return strings.Join(rows, "\n")
}

// copyAll returns the entire buffer as plain text (ANSI stripped). It copies the
// raw line set, not the filtered view, so an active filter never hides lines from
// the clipboard.
func (p *pager) copyAll() string {
	rows := make([]string, len(p.lines))
	for i, ln := range p.lines {
		rows[i] = ansi.Strip(ln)
	}
	return strings.Join(rows, "\n")
}

// clear empties the buffer and the viewport, re-enabling follow so a cleared
// view tracks whatever streams back in next. The filter is left intact.
func (p *pager) clear() {
	p.lines = nil
	p.filtered = nil
	p.matched = 0
	p.follow = true
	p.vp.SetContentLines(nil)
	p.stickToBottom()
}

// rebuildContent recomputes the filtered view from scratch, applying the active
// filter. Used when the line set or the filter changes.
func (p *pager) rebuildContent() {
	if p.re == nil {
		// Clone so filtered doesn't alias the lines backing array; storeLine
		// appends to each independently and a front-trim reslices lines.
		p.filtered = slices.Clone(p.lines)
		p.matched = len(p.filtered)
		return
	}
	p.filtered = p.filtered[:0]
	for _, ln := range p.lines {
		if p.re.MatchString(ansi.Strip(ln)) {
			p.filtered = append(p.filtered, ln)
		}
	}
	p.matched = len(p.filtered)
}

func (p *pager) filterActive() bool { return p.filter.Value() != "" }

func (p *pager) clearFilter() {
	p.filtering = false
	p.filter.Blur()
	p.filter.SetValue("")
	p.re = nil
	p.relayout()
}

func (p *pager) startFilter() {
	p.filtering = true
	p.filter.Focus()
	p.relayout()
}

// stopFilter exits filter mode. If clear is true the pattern is dropped and the
// full content restored.
func (p *pager) stopFilter(clear bool) {
	p.filtering = false
	p.filter.Blur()
	if clear {
		p.filter.SetValue("")
		p.applyFilter()
	}
	p.relayout()
}

// applyFilter compiles the current pattern and rebuilds the view. An empty or
// invalid pattern leaves re nil, so everything shows while the user keeps
// typing; filterLine flags the invalid case.
func (p *pager) applyFilter() {
	p.re = nil
	if val := p.filter.Value(); val != "" {
		if re, err := regexp.Compile(val); err == nil {
			p.re = re
		}
	}
	p.rebuildContent()
	p.syncViewport()
}

// toggleWrap switches between wrapping long lines (full line visible) and
// truncating them (one row per entry, scrollable left/right).
//
// YOffset means different things per mode: wrapped rows when wrapping, logical
// lines when not. Flipping the flag alone leaves a stale offset that can point
// past the content in the new mode, which renders a blank view until the next
// scroll re-clamps it. So pin the top visible line across the switch and set a
// valid offset for it in the new mode.
func (p *pager) toggleWrap() {
	top := p.lineAtRow(0)
	p.vp.SoftWrap = !p.vp.SoftWrap
	p.vp.SetYOffset(p.wrappedRowOf(top))
	p.stickToBottom()
}

// update feeds a message to the filter input while filtering (re-applying the
// pattern on change) or to the viewport otherwise. It mutates in place and
// returns any command the child produced.
func (p *pager) update(msg tea.Msg) tea.Cmd {
	if p.filtering {
		prev := p.filter.Value()
		var cmd tea.Cmd
		p.filter, cmd = p.filter.Update(msg)
		if p.filter.Value() != prev {
			p.applyFilter()
		}
		return cmd
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return cmd
}

// view renders the title (with right, a caller-supplied status), the filter row
// when active, and the viewport.
func (p pager) view(right string) string {
	title := p.th.ModalTitle.Render(p.title)
	header := spread(title, right, p.vp.Width())
	if p.filtering || p.filterActive() {
		return header + "\n" + p.filterLine() + "\n" + p.vp.View()
	}
	return header + "\n" + p.vp.View()
}

// selStatus is the right-hand header label while selecting, shared by every
// pager screen. ok is false when not selecting, so callers show their own status.
func (p pager) selStatus() (string, bool) {
	if !p.selecting {
		return "", false
	}
	if p.marking {
		return p.th.HeaderVal.Render(fmt.Sprintf("● mark %d", p.selCount())), true
	}
	return p.th.HeaderVal.Render("● select"), true
}

// filterLine renders the filter input with a match count, or an error marker
// when the pattern doesn't compile.
func (p pager) filterLine() string {
	var meta string
	switch {
	case p.re != nil:
		meta = p.th.Dim.Render(fmt.Sprintf("%d/%d", p.matched, len(p.lines)))
	case p.filterActive(): // text present but it didn't compile
		meta = p.th.Warn.Render("invalid regex")
	}
	return spread(p.filter.View(), meta, p.vp.Width())
}
