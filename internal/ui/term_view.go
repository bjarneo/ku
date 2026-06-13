package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"github.com/bjarneo/kli/internal/k8s"
)

const termFPS = 24

var cursorStyle = lipgloss.NewStyle().Reverse(true)

// termResult carries the session outcome from the streaming goroutine back to
// the model via waitTermDone.
type termResult struct {
	done chan struct{}
	err  error
}

// termView is an embedded terminal panel: a virtual-terminal emulator wired to
// either a pod exec stream or a local editor process. The transport writes the
// program's output into the emulator and reads encoded keystrokes back out,
// while the panel renders the emulator screen (with a cursor) each frame.
type termView struct {
	th      Theme
	em      *vt.SafeEmulator
	cancel  func()
	closeFn func()               // transport-specific teardown (queue/pty close)
	resize  func(cols, rows int) // push size to the transport
	onClose tea.Cmd              // optional cleanup run when the session ends (e.g. delete a debug pod)
	result  *termResult
	input   chan termInput
	title   string
	session int
	cols    int
	rows    int

	finished bool
	status   string

	// Edit-session context: when set, the file is applied on exit.
	isEdit       bool
	editPath     string
	editOriginal string
	editNs       string
	editName     string
	editRes      k8s.ResourceInfo
	editCl       *k8s.Client
}

func newTermView(th Theme) termView {
	return termView{th: th}
}

// termDims computes the emulator size from the available body area, reserving
// space for the panel border and a one-line title.
func termDims(width, bodyH int) (cols, rows int) {
	cols = paneContentWidth(width)
	rows = paneContentHeight(bodyH) - 1
	if cols < 8 {
		cols = 8
	}
	if rows < 3 {
		rows = 3
	}
	return cols, rows
}

func (t *termView) setSize(width, bodyH int) {
	cols, rows := termDims(width, bodyH)
	t.cols, t.rows = cols, rows
	if t.em != nil {
		t.em.Resize(cols, rows)
	}
	if t.resize != nil {
		t.resize(cols, rows)
	}
}

func (t *termView) stop() {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.em != nil {
		t.em.Close()
	}
	if t.closeFn != nil {
		t.closeFn()
	}
}

func (t termView) View(width, bodyH int) string {
	th := t.th
	cols := t.cols

	hint := "ctrl+\\ detach"
	if t.isEdit {
		hint = "save & quit to apply · ctrl+\\ cancel"
	}
	if t.finished {
		hint = t.status
	}
	hintR := th.Dim.Render(hint)
	title := th.ModalTitle.Render(truncate("● "+t.title, cols-lipgloss.Width(hintR)-1))
	titleLine := spread(title, hintR, cols)

	inner := titleLine + "\n" + t.renderScreen()
	box := th.PaneActive.Width(paneStyleWidth(width)).Height(paneStyleHeight(bodyH)).Render(inner)
	return lipgloss.Place(width, bodyH, lipgloss.Center, lipgloss.Center, box)
}

// renderScreen renders the emulator's screen, overlaying a block cursor onto
// the rendered string (never mutating emulator state, which the output writer
// goroutine owns concurrently).
func (t termView) renderScreen() string {
	if t.em == nil {
		return strings.Repeat("\n", t.rows-1)
	}
	screen := t.em.Render()
	lines := strings.Split(screen, "\n")
	if len(lines) > t.rows {
		lines = lines[:t.rows]
	}
	for len(lines) < t.rows {
		lines = append(lines, "")
	}

	if !t.finished {
		pos := t.em.CursorPosition()
		if pos.Y >= 0 && pos.Y < len(lines) && pos.X >= 0 && pos.X < t.cols {
			lines[pos.Y] = overlayCursor(lines[pos.Y], pos.X, t.cols)
		}
	}
	return strings.Join(lines, "\n")
}

// overlayCursor draws a reverse-video block at display column col on a rendered
// (possibly styled) line.
func overlayCursor(line string, col, width int) string {
	before := ansi.Cut(line, 0, col)
	if w := ansi.StringWidth(before); w < col {
		before += strings.Repeat(" ", col-w)
	}
	ch := ansi.Strip(ansi.Cut(line, col, col+1))
	if ch == "" {
		ch = " "
	}
	after := ansi.Cut(line, col+1, width)
	return before + cursorStyle.Render(ch) + after
}

func termTick(session int) tea.Cmd {
	return tea.Tick(time.Second/termFPS, func(time.Time) tea.Msg {
		return termTickMsg{session: session}
	})
}

func waitTermDone(session int, r *termResult) tea.Cmd {
	return func() tea.Msg {
		<-r.done
		return termDoneMsg{session: session, err: r.err}
	}
}
