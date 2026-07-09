package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDetailCopyStripsYAMLHighlighting(t *testing.T) {
	d := newDetailView(PickTheme("tokyonight"))
	d.setSize(80, 20)
	d.setYAML("pod/api", "apiVersion: v1\nkind: Pod\n")

	got := d.copyAll()
	if strings.Contains(got, "\x1b") {
		t.Fatalf("copyAll must strip ANSI highlighting, got %q", got)
	}
	if !strings.Contains(got, "kind: Pod") {
		t.Fatalf("copyAll should contain the plain yaml, got %q", got)
	}
}

func TestDetailRendersBorderlessForCleanCopy(t *testing.T) {
	a := App{theme: PickTheme("ansi"), width: 60, height: 20, screen: screenDetail}
	a.detail = newDetailView(a.theme)
	a.detail.setSize(pagerContentWidth(a.width), pagerContentHeight(a.bodyH()))
	a.detail.setYAML("pod/api", "kind: Pod\nmetadata:\n  name: api-7d9\n")

	out := a.renderPagerPane(a.detail.View(), a.width, a.bodyH())
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "name: api-7d9") && strings.Contains(ln, "│") {
			t.Fatalf("detail row carries a vertical border, native copy would grab it: %q", ln)
		}
	}
}

// The detail screen now shares the pager's mouse selection: a left-drag copies
// the selected lines and keeps them highlighted.
func TestDetailMouseDragCopies(t *testing.T) {
	a := App{theme: PickTheme("ansi"), width: 80, height: 24, screen: screenDetail, focus: focusMain, keys: defaultKeys()}
	a.detail = newDetailView(a.theme)
	a.detail.setSize(pagerContentWidth(a.width), pagerContentHeight(a.bodyH()))
	a.detail.setYAML("pod/api", "kind: Pod\nmetadata:\n  name: api\n  namespace: default\n")
	a.detail.vp.SoftWrap = false // 1 row per line for a predictable mapping
	a.detail.vp.GotoTop()

	var m tea.Model = a
	m, _ = m.Update(tea.MouseClickMsg{X: 1, Y: 3, Button: tea.MouseLeft}) // first content row
	m, _ = m.Update(tea.MouseMotionMsg{X: 1, Y: 4, Button: tea.MouseLeft})
	m, cmd := m.Update(tea.MouseReleaseMsg{X: 1, Y: 4, Button: tea.MouseLeft})
	got := m.(App)

	if cmd == nil {
		t.Fatal("dragging on the detail view should copy to the clipboard")
	}
	if !got.detail.selecting {
		t.Fatal("selection should stay highlighted after copy")
	}
	if !strings.Contains(got.status, "chars") {
		t.Fatalf("expected a copy status, got %q", got.status)
	}
}
