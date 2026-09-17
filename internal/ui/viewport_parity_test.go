package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
)

// pagerViewport replaces bubbles' viewport on the hot path, so it must render
// and scroll exactly like it. This test drives both with the same content,
// sizes, offsets and wrap modes and compares the output. Keep it green when
// either implementation changes; a mismatch is a user-visible regression.
func TestPagerViewportParityWithBubbles(t *testing.T) {
	cases := map[string][]string{
		"empty":       nil,
		"blank":       {""},
		"one short":   {"hello"},
		"ansi":        {"\x1b[31m" + strings.Repeat("a", 57) + "\x1b[0m"},
		"wide runes":  {strings.Repeat("wide字", 30)},
		"long plain":  {strings.Repeat("x", 300), "short", strings.Repeat("y", 91)},
		"trailing ws": {"padded     ", "  more   ", ""},
		"many":        makeManyLines(120),
	}
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 6; i++ {
		cases[fmt.Sprintf("random-%d", i)] = randomViewportLines(r)
	}

	for name, lines := range cases {
		for _, wrap := range []bool{true, false} {
			mine := newPagerViewport()
			mine.SoftWrap = wrap
			theirs := viewport.New()
			theirs.SoftWrap = wrap
			for _, w := range []int{1, 16, 40, 80} {
				mine.SetWidth(w)
				theirs.SetWidth(w)
				for _, h := range []int{1, 10} {
					mine.SetHeight(h)
					theirs.SetHeight(h)
					mine.SetContentLines(lines)
					theirs.SetContentLines(lines)

					if total, _, _ := mine.lineAtRow(0); total != theirs.TotalLineCount() {
						t.Fatalf("%s wrap=%v w=%d h=%d: total rows %d, bubbles %d", name, wrap, w, h, total, theirs.TotalLineCount())
					}
					maxOff := mine.maxYOffset()
					for _, off := range []int{0, 1, maxOff / 2, maxOff, maxOff + 5} {
						off = max(0, off)
						mine.SetYOffset(off)
						theirs.SetYOffset(off)
						if mine.YOffset() != theirs.YOffset() {
							t.Fatalf("%s wrap=%v w=%d h=%d off=%d: yoffset %d, bubbles %d", name, wrap, w, h, off, mine.YOffset(), theirs.YOffset())
						}
						if mv, tv := mine.View(), theirs.View(); mv != tv {
							t.Fatalf("%s wrap=%v w=%d h=%d off=%d:\nmine: %q\nbubbles: %q", name, wrap, w, h, off, mv, tv)
						}
						for _, xoff := range []int{0, 7, 25} {
							mine.SetXOffset(xoff)
							theirs.SetXOffset(xoff)
							if mv, tv := mine.View(), theirs.View(); mv != tv {
								t.Fatalf("%s wrap=%v w=%d h=%d off=%d xoff=%d:\nmine: %q\nbubbles: %q", name, wrap, w, h, off, xoff, mv, tv)
							}
						}
					}
				}
			}
		}
	}
}

func makeManyLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%03d %s", i, strings.Repeat("m", i%23))
	}
	return lines
}

func randomViewportLines(r *rand.Rand) []string {
	n := r.Intn(40)
	lines := make([]string, n)
	for i := range lines {
		switch r.Intn(5) {
		case 0:
			lines[i] = ""
		case 1:
			lines[i] = strings.Repeat("x", r.Intn(5))
		case 2:
			lines[i] = strings.Repeat("wide字", r.Intn(20))
		case 3:
			lines[i] = "\x1b[31m" + strings.Repeat("a", r.Intn(120)) + "\x1b[0m"
		default:
			lines[i] = fmt.Sprintf("line-%d %s", r.Intn(1000), strings.Repeat("q", r.Intn(90)))
		}
	}
	return lines
}
