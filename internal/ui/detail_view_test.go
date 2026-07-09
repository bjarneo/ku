package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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

func TestDetailFilterMatchesVisibleText(t *testing.T) {
	d := newDetailView(PickTheme("tokyonight"))
	d.setSize(80, 20)
	d.setYAML("pod/api", "apiVersion: v1\nkind: Pod\nmetadata:\n  name: api\n")
	d.startFilter()
	d.filter.SetValue("^kind: Pod$")
	d.applyFilter()

	if d.matched != 1 {
		t.Fatalf("highlighted YAML filter matched %d lines, want 1", d.matched)
	}
	if got := d.copyAll(); !strings.Contains(got, "kind: Pod") {
		t.Fatalf("precondition failed: copyAll missing visible text: %q", got)
	}
}

func TestPagerFilterCapturesGlobalKeys(t *testing.T) {
	a := App{theme: PickTheme("ansi"), keys: defaultKeys(), screen: screenDetail, readOnly: true}
	a.detail = newDetailView(a.theme)
	a.detail.setSize(80, 20)
	a.detail.setYAML("pod/api", "kind: Pod\n")
	a.detail.startFilter()

	m, cmd := a.handleKey(mkKey("C"))
	got := m.(App)
	if cmd != nil {
		t.Fatal("filter input returned command for text key")
	}
	if got.overlay != overlayNone {
		t.Fatalf("global command opened while filtering: overlay=%v", got.overlay)
	}
	if got.detail.filter.Value() != "C" {
		t.Fatalf("filter did not capture C key, got %q", got.detail.filter.Value())
	}
}

func TestPagerSelectionCapturesGlobalKeys(t *testing.T) {
	a := App{theme: PickTheme("ansi"), keys: defaultKeys(), screen: screenDetail, readOnly: true}
	a.detail = newDetailView(a.theme)
	a.detail.setSize(80, 20)
	a.detail.setYAML("pod/api", "kind: Pod\n")
	a.detail.startSelect()

	m, _ := a.handleKey(mkKey("E"))
	got := m.(App)
	if !got.readOnly {
		t.Fatal("edit mode toggled while pager selection was active")
	}
	if !got.detail.selecting {
		t.Fatal("selection should still be active after unrelated global key")
	}
}

func TestDetailContentReplacementClearsSelectionAndFilter(t *testing.T) {
	d := newDetailView(PickTheme("ansi"))
	d.setSize(80, 20)
	d.setYAML("pod/old", "kind: Pod\n")
	d.startFilter()
	d.filter.SetValue("kind")
	d.applyFilter()
	d.startSelect()

	d.setYAML("pod/new", "apiVersion: v1\n")

	if d.selecting {
		t.Fatal("content replacement should clear stale selection")
	}
	if d.filterActive() || d.filtering || d.re != nil {
		t.Fatalf("content replacement should clear stale filter: active=%t filtering=%t re=%v", d.filterActive(), d.filtering, d.re)
	}
}

func TestDetailKeepsPaneBorders(t *testing.T) {
	a := App{theme: PickTheme("ansi"), width: 60, height: 20, screen: screenDetail}
	a.detail = newDetailView(a.theme)
	a.detail.setSize(paneContentWidth(a.width), paneContentHeight(a.bodyH()))
	a.detail.setYAML("pod/api", "kind: Pod\nmetadata:\n  name: api-7d9\n")

	out := a.renderPane(a.theme.PaneActive, a.detail.View(), a.width, a.bodyH())
	for _, ln := range strings.Split(out, "\n") {
		plain := ansi.Strip(ln)
		if strings.Contains(plain, "name: api-7d9") && strings.Contains(plain, "│") {
			return
		}
	}
	t.Fatalf("detail row should keep the pane border: %q", out)
}
