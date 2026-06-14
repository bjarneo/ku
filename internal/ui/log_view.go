package ui

import (
	"bufio"
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bjarneo/kli/internal/k8s"
)

const maxLogLines = 5000

// logView streams a pod container's logs into a viewport. The stream always
// follows server-side; the follow flag controls whether new lines auto-scroll
// to the bottom (so the user can scroll up to read history).
type logView struct {
	th     Theme
	vp     viewport.Model
	title  string
	ns     string
	pod    string
	cont   string
	deploy string
	follow bool

	session int
	streams int
	cancel  context.CancelFunc
	ch      chan logEvent
	lines   []string
	content string // joined view content, maintained incrementally
}

func newLogView(th Theme) logView {
	return logView{th: th, vp: viewport.New(0, 0), follow: true}
}

func (l *logView) setSize(w, h int) {
	if h < 1 {
		h = 1
	}
	l.vp.Width = w
	l.vp.Height = h - 1
}

func (l *logView) appendLine(s string) {
	l.lines = append(l.lines, s)
	switch {
	case len(l.lines) > maxLogLines:
		l.lines = l.lines[len(l.lines)-maxLogLines:]
		l.content = strings.Join(l.lines, "\n") // rebuild only when trimming the front
	case l.content == "":
		l.content = s
	default:
		l.content += "\n" + s
	}
	l.vp.SetContent(l.content)
	if l.follow {
		l.vp.GotoBottom()
	}
}

func (l *logView) stop() {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
}

func (l logView) Update(msg tea.Msg) (logView, tea.Cmd) {
	var cmd tea.Cmd
	l.vp, cmd = l.vp.Update(msg)
	return l, cmd
}

func (l logView) View() string {
	state := "following"
	style := l.th.Good
	if !l.follow {
		state = "paused"
		style = l.th.Warn
	}
	right := style.Render("● " + state)
	title := l.th.ModalTitle.Render(l.title)
	return spread(title, right, l.vp.Width) + "\n" + l.vp.View()
}

// streamLogs opens the log stream and feeds lines onto ch until the context is
// canceled or the stream ends. It sends a done event unless cancellation already
// made that event irrelevant.
func streamLogs(ctx context.Context, cl *k8s.Client, ns, pod, cont, prefix string, session int, ch chan logEvent) {
	defer func() {
		select {
		case ch <- logEvent{session: session, done: true}:
		case <-ctx.Done():
		}
	}()

	rc, err := cl.LogStream(ctx, ns, pod, cont, logTailLines, true)
	if err != nil {
		select {
		case ch <- logEvent{session: session, err: err}:
		case <-ctx.Done():
		}
		return
	}
	defer rc.Close()

	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if prefix != "" {
			line = prefix + " | " + line
		}
		select {
		case <-ctx.Done():
			return
		case ch <- logEvent{session: session, line: line}:
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		select {
		case ch <- logEvent{session: session, err: err}:
		case <-ctx.Done():
		}
	}
}

func waitForLog(ch chan logEvent) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
