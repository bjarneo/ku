package ui

import (
	"fmt"
	"testing"
)

// benchLogLine approximates a structured JSON log line, the workload that made
// the pager slow as the buffer filled.
func benchLogLine(i int) string {
	return fmt.Sprintf(
		`{"ts":"2026-09-02T18:21:%02d.%03dZ","level":"info","logger":"example-service-grpc","caller":"grpc/interceptor.go:118","msg":"unary call finished","method":"/example.v1.ExampleService/Get","peer":"10.42.3.%d:5%04d","code":"OK","duration_ms":%d,"request_id":"7f3a9c2e-%04d-4b1d-9e7a-1c2d3e4f5a6b"}`,
		i%60, i%1000, i%255, i%10000, i%400, i%10000)
}

func benchPagerAtCap(b *testing.B, softWrap bool) *pager {
	b.Helper()
	p := newPager(PickTheme("ansi"))
	p.setSize(180, 48)
	p.vp.SoftWrap = softWrap
	for i := range maxLogLines {
		p.storeLine(benchLogLine(i))
	}
	p.syncViewport()
	return &p
}

// One iteration is one 50-line stream batch plus the frame a log update draws.
func BenchmarkLogPagerStreamBatchAtCap(b *testing.B) {
	for _, wrap := range []bool{true, false} {
		b.Run(fmt.Sprintf("wrap=%v", wrap), func(b *testing.B) {
			p := benchPagerAtCap(b, wrap)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := range 50 {
					p.storeLine(benchLogLine(maxLogLines + i*50 + j))
				}
				p.syncViewport()
				_ = p.view("x")
			}
		})
	}
}

// An idle frame with no new lines: keystrokes and the refresh tick pay this.
func BenchmarkLogPagerIdleFrameAtCap(b *testing.B) {
	p := benchPagerAtCap(b, true)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = p.view("x")
	}
}
