package ui

import (
	"bufio"
	"context"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bjarneo/ku/internal/k8s"
)

// logView streams a pod container's logs into a pager. The stream always follows
// server-side; the follow flag (on the pager) controls whether new lines
// auto-scroll to the bottom, so the user can scroll up to read history.
type logView struct {
	pager

	ns     string
	pod    string
	cont   string
	deploy string

	session int
	streams int
	cancel  context.CancelFunc
	ch      chan logEvent
}

func newLogView(th Theme) logView {
	return logView{pager: newPager(th)}
}

func (l *logView) stop() {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
}

func (l logView) View() string {
	right, ok := l.selStatus()
	if !ok {
		mode := "wrap"
		if !l.vp.SoftWrap {
			mode = "nowrap"
		}
		state, style := "following", l.th.Good
		if !l.follow {
			state, style = "paused", l.th.Warn
		}
		right = l.th.Dim.Render(mode) + "  " + style.Render("● "+state)
	}
	return l.view(right)
}

// streamLogs opens the log stream and feeds lines onto ch until the context is
// canceled or the stream ends. It sends a done event unless cancellation already
// made that event irrelevant.
func streamLogs(ctx context.Context, cl *k8s.Client, ns, pod, cont, prefix string, session int, ch chan logEvent) {
	defer func() {
		sendLogEvent(ch, logEvent{session: session, done: true})
	}()

	rc, err := cl.LogStream(ctx, ns, pod, cont, logTailLines, true)
	if err != nil {
		sendLogEvent(ch, logEvent{session: session, err: err})
		return
	}
	defer rc.Close()

	// A bufio.Reader (not Scanner) keeps lines intact at any length: ReadString
	// grows past the 64KB read buffer until it hits the newline, where a Scanner
	// would error out and drop the line once it crossed its token cap.
	br := bufio.NewReaderSize(rc, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if prefix != "" {
				line = prefix + " | " + line
			}
			select {
			case <-ctx.Done():
				return
			case ch <- logEvent{session: session, line: line}:
			}
		}
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				sendLogEvent(ch, logEvent{session: session, err: err})
			}
			return
		}
	}
}

func sendLogEvent(ch chan<- logEvent, ev logEvent) {
	select {
	case ch <- ev:
	default:
	}
}

func waitForLog(ch chan logEvent) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
