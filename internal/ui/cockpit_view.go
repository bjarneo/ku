package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/ku/internal/k8s"
)

// cockpitView renders the cluster overview dashboard shown on launch.
type cockpitView struct {
	th    Theme
	data  *k8s.ClusterOverview
	ready bool
}

func newCockpitView(th Theme) cockpitView { return cockpitView{th: th} }

func (c *cockpitView) setData(o *k8s.ClusterOverview) {
	c.data = o
	c.ready = true
}

// View renders the dashboard to exactly width x height, clamping so it can
// never break the surrounding layout.
func (c cockpitView) View(width, height int) string {
	if !c.ready || c.data == nil {
		return clampBlock(c.th.Dim.Render("  loading cluster overview…"), width, height)
	}
	o := c.data
	th := c.th

	cluster := []string{
		c.kv("version", valOrDash(o.Version)),
		c.kv("nodes", fmt.Sprintf("%d/%d ready", o.NodesReady, o.Nodes)),
		c.kv("namespaces", fmt.Sprintf("%d", o.Namespaces)),
	}
	if len(o.NodeIssues) > 0 {
		msg := o.NodeIssues[0]
		if extra := len(o.NodeIssues) - 1; extra > 0 {
			msg += fmt.Sprintf(" +%d", extra)
		}
		cluster = append(cluster, th.Bad.Render("⚠ "+msg))
	}

	workloads := []string{
		c.kv("pods", fmt.Sprintf("%d", o.Pods)),
		"  " + th.Good.Render(fmt.Sprintf("%d running", o.PodRunning)),
		"  " + th.Warn.Render(fmt.Sprintf("%d pending", o.PodPending)),
		"  " + th.Bad.Render(fmt.Sprintf("%d failed", o.PodFailed)),
	}
	if o.PodNotReady > 0 {
		workloads = append(workloads, "  "+th.Warn.Render(fmt.Sprintf("%d not ready", o.PodNotReady)))
	}
	if o.PodCrashLoop > 0 {
		workloads = append(workloads, "  "+th.Bad.Render(fmt.Sprintf("%d crashloop", o.PodCrashLoop)))
	}
	workloads = append(workloads, c.kv("deploys", fmt.Sprintf("%d/%d ready", o.DeploymentsReady, o.Deployments)))

	// Top region: three columns when wide, stacked otherwise. Resources holds
	// the widest lines, so it takes a larger share. Gauges are sized per pane,
	// so the Resources lines are built once the pane width is known.
	var top string
	topH := clamp(height/2, 5, 9)
	gap := strings.Repeat(" ", paneGap)
	if width >= 72 {
		panelW := width - 2*paneGap
		w1 := panelW * 27 / 100
		w3 := w1
		w2 := panelW - w1 - w3
		top = lipgloss.JoinHorizontal(lipgloss.Top,
			c.panel("Cluster", cluster, w1, topH), gap,
			c.panel("Resources", c.resourceLines(w2), w2, topH), gap,
			c.panel("Workloads", workloads, w3, topH),
		)
	} else {
		resources := c.resourceLines(width)
		top = lipgloss.JoinVertical(lipgloss.Left,
			c.panel("Cluster", cluster, width, 5),
			c.panel("Resources", resources, width, len(resources)+3),
			c.panel("Workloads", workloads, width, 7),
		)
		topH = lipgloss.Height(top)
	}

	warnH := height - topH
	if warnH < 3 {
		warnH = 3
	}
	warnings := c.panel("Recent warnings", c.warningLines(paneContentWidth(width)), width, warnH)

	return clampBlock(lipgloss.JoinVertical(lipgloss.Left, top, warnings), width, height)
}

// resourceLines renders CPU and memory as one bar each with three segments:
// live usage, requested but idle, and free to schedule. Requests are what the
// scheduler books against a node, so they differ from live usage: a cluster
// can be idle yet fully booked. The legend below the bars carries the values.
// Pods waiting for a node add a pending line with the demand nothing can place.
func (c cockpitView) resourceLines(paneW int) []string {
	o := c.data
	th := c.th
	if o.CPUAllocMilli <= 0 && o.MemAllocBytes <= 0 {
		return []string{th.Dim.Render("capacity unavailable")}
	}
	gw := clamp(paneContentWidth(paneW), 12, 44)
	cpuFree := max(o.CPUAllocMilli-o.CPUReqMilli, 0)
	memFree := max(o.MemAllocBytes-o.MemReqBytes, 0)

	legend := func(glyph string, style lipgloss.Style, key, val string) string {
		return style.Render(glyph) + " " + th.HeaderKey.Render(fmt.Sprintf("%-10s", key)) + val
	}
	lines := []string{
		c.gauge("CPU", pct(o.CPUUsedMilli, o.CPUAllocMilli), pct(o.CPUReqMilli, o.CPUAllocMilli), o.HasMetrics, gw),
		c.gauge("MEM", pct(o.MemUsedBytes, o.MemAllocBytes), pct(o.MemReqBytes, o.MemAllocBytes), o.HasMetrics, gw),
	}
	if o.HasMetrics {
		lines = append(lines, legend("█", th.Good, "used", fmt.Sprintf("%s cores · %s", cores(o.CPUUsedMilli), gib(o.MemUsedBytes))))
	} else {
		lines = append(lines, legend("█", th.Dim, "used", th.Dim.Render("metrics unavailable")))
	}
	lines = append(lines,
		legend("▓", th.HeaderVal, "requested", fmt.Sprintf("%s cores · %s", cores(o.CPUReqMilli), gib(o.MemReqBytes))),
		legend("░", th.Dim, "free", fmt.Sprintf("%s/%s cores · %.1f/%s",
			cores(cpuFree), cores(o.CPUAllocMilli), float64(memFree)/(1<<30), gib(o.MemAllocBytes))),
	)
	if o.CPUPendingMilli > 0 || o.MemPendingBytes > 0 {
		lines = append(lines, legend("⚠", th.Bad, "pending",
			th.Bad.Render(fmt.Sprintf("%s cores · %s", cores(o.CPUPendingMilli), gib(o.MemPendingBytes)))))
	}
	return lines
}

func (c cockpitView) warningLines(w int) []string {
	o := c.data
	th := c.th
	if len(o.Warnings) == 0 {
		return []string{th.Good.Render("no recent warnings")}
	}
	lines := make([]string, 0, len(o.Warnings))
	for _, e := range o.Warnings {
		loc := e.Object
		if e.Namespace != "" {
			loc = e.Namespace + "/" + e.Object
		}
		reason := e.Reason
		if e.Count > 1 {
			reason += fmt.Sprintf(" ×%d", e.Count)
		}
		left := fmt.Sprintf("%-4s ", e.Age) + th.HeaderVal.Render(truncate(loc, 36)) + " " + th.Warn.Render(reason)
		msg := th.Dim.Render(e.Message)
		line := left + "  " + msg
		lines = append(lines, ansi.Truncate(line, w, "…"))
	}
	return lines
}

func (c cockpitView) kv(k, v string) string {
	return c.th.HeaderKey.Render(fmt.Sprintf("%-11s", k)) + v
}

// gauge renders "LABEL [████▓▓░░░░] 67%": used (colored by utilization),
// requested but idle (accent), and free (dim). The trailing percentage is live
// usage; without metrics the used segment is empty and the percentage is the
// requested share instead.
func (c cockpitView) gauge(label string, usedPct, reqPct int, hasMetrics bool, width int) string {
	th := c.th
	// layout: "LABEL " + bar + " 100%"  => label + 1 + barW + 1 + 4
	barW := width - len(label) - 6
	if barW < 4 {
		barW = 4
	}
	if !hasMetrics {
		usedPct = 0
	}
	used := clamp(barW*usedPct/100, 0, barW)
	req := clamp(barW*reqPct/100, used, barW)
	style := th.Good
	switch {
	case usedPct >= 85:
		style = th.Bad
	case usedPct >= 70:
		style = th.Warn
	}
	bar := style.Render(strings.Repeat("█", used)) +
		th.HeaderVal.Render(strings.Repeat("▓", req-used)) +
		th.Dim.Render(strings.Repeat("░", barW-req))
	shown := usedPct
	if !hasMetrics {
		shown = reqPct
	}
	return fmt.Sprintf("%s %s %3d%%", label, bar, shown)
}

// panel draws a titled, bordered box of exactly outerW x outerH, clipping its
// content (which may be styled) to fit.
func (c cockpitView) panel(title string, lines []string, outerW, outerH int) string {
	innerW := paneContentWidth(outerW)
	innerH := paneContentHeight(outerH)
	content := []string{c.th.ModalTitle.Render(truncate(title, innerW))}
	for _, ln := range lines {
		if len(content) >= innerH {
			break
		}
		content = append(content, ansi.Truncate(ln, innerW, "…"))
	}
	return c.th.PaneInactive.Width(paneStyleWidth(outerW)).Height(paneStyleHeight(outerH)).Render(strings.Join(content, "\n"))
}

// --- helpers ---------------------------------------------------------------

func pct(used, total int64) int {
	if total <= 0 {
		return 0
	}
	return clamp(int(used*100/total), 0, 100)
}

func cores(milli int64) string { return fmt.Sprintf("%.1f", float64(milli)/1000) }

func gib(b int64) string { return fmt.Sprintf("%.1fGi", float64(b)/(1024*1024*1024)) }

func valOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// clampBlock forces a rendered block to exactly width x height lines.
func clampBlock(s string, width, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
