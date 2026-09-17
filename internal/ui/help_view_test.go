package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"charm.land/lipgloss/v2"
)

// A note wider than the terminal would set the box width, costing the modal its
// right border and the note its tail to the overlay clamp.
func TestHelpViewWrapsLongModeNote(t *testing.T) {
	h := newHelpView(PickTheme("ansi"), defaultKeys())
	h.note = "Developer + read-only mode: nav scoped to app resources; all writes disabled"

	for _, width := range []int{60, 80, 100, 132} {
		got := ansi.Strip(h.View(width, 30))
		if w := lipgloss.Width(got); w > width {
			t.Errorf("width %d: View() rendered %d columns wide", width, w)
		}
		if !strings.Contains(got, "Developer + read-only mode") {
			t.Errorf("width %d: View() = %q, want the mode note", width, got)
		}
		// The note wraps, so check the last word rather than a phrase that a
		// line break can split.
		if !strings.Contains(got, "disabled") {
			t.Errorf("width %d: View() lost the tail of the mode note", width)
		}
	}
}

// The note takes rows away from the keybinding grid, so a wrapped note has to
// shrink the grid by its own line count or the box outgrows the overlay.
func TestHelpViewNoteHeightAccountsForWrapping(t *testing.T) {
	th := PickTheme("ansi")
	keys := defaultKeys()
	plain := newHelpView(th, keys)
	wrapped := newHelpView(th, keys)
	wrapped.note = "Developer + read-only mode: nav scoped to app resources; all writes disabled"

	const width, height = 80, 30
	plainRows := strings.Count(plain.View(width, height), "\n")
	wrappedRows := strings.Count(wrapped.View(width, height), "\n")
	if wrappedRows != plainRows {
		t.Fatalf("note changed the box height: %d rows with a note, %d without", wrappedRows, plainRows)
	}
}
