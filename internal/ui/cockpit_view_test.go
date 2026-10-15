package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestCockpitResourcesShowsUsageRequestsAndFree(t *testing.T) {
	v := newCockpitView(PickTheme("ansi"))
	v.setData(fakeOverview())

	for _, width := range []int{140, 100, 60} {
		out := ansi.Strip(v.View(width, 30))
		for _, want := range []string{
			"used      4.2 cores · 12.0Gi",
			"requested 9.0 cores · 20.0Gi",
			"free      7.0/16.0 cores · 12.0/32.0Gi",
			"pending   0.5 cores · 1.0Gi",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("width %d: output missing %q:\n%s", width, want, out)
			}
		}
		for i, line := range strings.Split(out, "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width %d: line %d is %d wide", width, i, w)
			}
		}
	}
}

func TestCockpitResourcesHidesPendingWhenNone(t *testing.T) {
	o := fakeOverview()
	o.CPUPendingMilli, o.MemPendingBytes = 0, 0
	v := newCockpitView(PickTheme("ansi"))
	v.setData(o)
	if out := ansi.Strip(v.View(140, 30)); strings.Contains(out, "0.5 cores · 1.0Gi") {
		t.Fatalf("unexpected pending line:\n%s", out)
	}
}

func TestCockpitResourcesWithoutMetricsStillShowsRequests(t *testing.T) {
	o := fakeOverview()
	o.HasMetrics = false
	v := newCockpitView(PickTheme("ansi"))
	v.setData(o)
	out := ansi.Strip(v.View(140, 30))
	for _, want := range []string{"metrics unavailable", "free      7.0/16.0 cores"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}

func TestCockpitResourcesUnavailableWithoutNodes(t *testing.T) {
	v := newCockpitView(PickTheme("ansi"))
	v.setData(&k8s.ClusterOverview{})
	if out := ansi.Strip(v.View(140, 30)); !strings.Contains(out, "capacity unavailable") {
		t.Fatalf("expected placeholder:\n%s", out)
	}
}

func TestGaugeSegments(t *testing.T) {
	v := newCockpitView(PickTheme("ansi"))
	got := ansi.Strip(v.gauge("CPU", 25, 50, true, 30))
	// barW = 30 - 3 - 6 = 21: 5 used, 5 requested-idle, 11 free.
	want := "CPU █████▓▓▓▓▓░░░░░░░░░░░  25%"
	if got != want {
		t.Fatalf("gauge() = %q, want %q", got, want)
	}
	if got := ansi.Strip(v.gauge("CPU", 25, 50, false, 30)); got != "CPU ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░  50%" {
		t.Fatalf("gauge() without metrics = %q", got)
	}
}
