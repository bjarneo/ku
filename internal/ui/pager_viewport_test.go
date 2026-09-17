package ui

import (
	"fmt"
	"strings"
	"testing"
)

// The pager pushes a stream with AppendLines between full replaces. Both paths
// must leave the viewport in the same state.
func TestPagerViewportAppendMatchesSet(t *testing.T) {
	lines := make([]string, 0, 300)
	for i := range 300 {
		switch i % 4 {
		case 0:
			lines = append(lines, "")
		case 1:
			lines = append(lines, fmt.Sprintf("line-%d", i))
		case 2:
			lines = append(lines, strings.Repeat("long ", 40))
		default:
			lines = append(lines, "short")
		}
	}

	for _, wrap := range []bool{true, false} {
		set := newPagerViewport()
		set.SoftWrap = wrap
		set.SetWidth(37)
		set.SetHeight(9)
		set.SetContentLines(lines)

		app := newPagerViewport()
		app.SoftWrap = wrap
		app.SetWidth(37)
		app.SetHeight(9)
		for i := 0; i < len(lines); i += 17 {
			app.AppendLines(lines[i:min(i+17, len(lines))])
		}

		setTotal, _, _ := set.lineAtRow(0)
		appTotal, _, _ := app.lineAtRow(0)
		if setTotal != appTotal {
			t.Fatalf("wrap=%v: appended total %d, set total %d", wrap, appTotal, setTotal)
		}
		for off := 0; off <= set.maxYOffset()+3; off++ {
			set.SetYOffset(off)
			app.SetYOffset(off)
			if got, want := app.View(), set.View(); got != want {
				t.Fatalf("wrap=%v off=%d:\nappend: %q\nset:    %q", wrap, off, got, want)
			}
		}
	}
}

func TestPagerViewportLineAtRow(t *testing.T) {
	v := newPagerViewport()
	v.SoftWrap = true
	v.SetWidth(10)
	v.SetHeight(4)
	// Widths 10, 25, 0: rows 1, 3, 1 for a total of 5.
	v.SetContentLines([]string{strings.Repeat("a", 10), strings.Repeat("b", 25), ""})

	if total, _, _ := v.lineAtRow(0); total != 5 {
		t.Fatalf("total rows = %d, want 5", total)
	}
	for _, tc := range []struct{ row, line, inner int }{
		{0, 0, 0}, {1, 1, 0}, {2, 1, 1}, {3, 1, 2}, {4, 2, 0}, {99, 3, 0},
	} {
		_, line, inner := v.lineAtRow(tc.row)
		if line != tc.line || inner != tc.inner {
			t.Errorf("lineAtRow(%d) = line %d row %d; want line %d row %d", tc.row, line, inner, tc.line, tc.inner)
		}
	}
}

func TestPagerViewportScrollPercent(t *testing.T) {
	v := newPagerViewport()
	v.SetWidth(20)
	v.SetHeight(10)
	v.SetContentLines(make([]string, 20))

	if got := v.ScrollPercent(); got != 0 {
		t.Fatalf("top: percent = %v, want 0", got)
	}
	v.GotoBottom()
	if got := v.ScrollPercent(); got != 1 {
		t.Fatalf("bottom: percent = %v, want 1", got)
	}

	v2 := newPagerViewport()
	v2.SetWidth(20)
	v2.SetHeight(10)
	v2.SetContentLines(make([]string, 3))
	if got := v2.ScrollPercent(); got != 1 {
		t.Fatalf("short content: percent = %v, want 1", got)
	}
}

// At the cap the pager drops a whole chunk, so the filter rescan and viewport
// rebuild run once per chunk instead of once per line.
func TestPagerTrimsInChunksAtCap(t *testing.T) {
	l := newLogView(PickTheme("ansi"))
	l.setSize(80, 24)
	setFilter(&l, "keep")
	l.storeLine("keep 0")
	l.syncViewport()

	for i := 1; i <= maxLogLines+logTrimChunk; i++ {
		l.storeLine(fmt.Sprintf("keep %d", i))
	}
	l.syncViewport()

	if got := len(l.lines); got > maxLogLines {
		t.Fatalf("trimmed buffer holds %d lines, want at most %d", got, maxLogLines)
	}
	if got := len(l.lines); got < maxLogLines-logTrimChunk {
		t.Fatalf("trim dropped too much: %d lines left", got)
	}
	if l.matched != len(l.lines) {
		t.Fatalf("filtered count %d, raw count %d", l.matched, len(l.lines))
	}
	if lines := l.vp.lines; len(lines) != l.matched {
		t.Fatalf("viewport holds %d rows, want %d", len(lines), l.matched)
	}
	if len(l.lines) == 0 || l.vp.lines[0] != l.lines[0] {
		t.Fatalf("viewport is out of sync with the buffer: %q vs %q", l.vp.lines[0], l.lines[0])
	}
}
