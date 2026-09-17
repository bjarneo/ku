package ui

import "testing"

func TestCellLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool // want a < b
	}{
		{"2h", "5d", true},      // 2h younger than 5d
		{"5d", "2h", false},     // and not the reverse
		{"90m", "2h", true},     // 90m (5400s) < 2h (7200s)
		{"1h30m", "100m", true}, // 5400s < 6000s
		// Durations sort by elapsed time in any column (e.g. LAST SEEN in
		// Events), not by leading number or lexically.
		{"2s", "2m8s", true},
		{"2m59s", "20m", true},
		{"20m", "3h", true},
		{"3h", "20m", false},
		{"100m", "2000m", true}, // CPU: numeric, not lexical ("2000m" < "100m" lexically)
		{"512Mi", "2048Mi", true},
		{"9%", "80%", true}, // numeric, not lexical
		{"2", "10", true},   // restarts
		{"1/1", "10/10", true},
		{"alpha", "beta", true},
		{"Zeta", "alpha", false}, // case-insensitive: z > a
		{"Running", "Pending", false},
		// Names with embedded numbers sort by value, not lexically.
		{"node-2", "node-10", true},
		{"node-10", "node-2", false},
		{"worker-9.example.com", "worker-10.example.com", true},
		{"ip-10-0-1-9.ec2.internal", "ip-10-0-1-23.ec2.internal", true},
		{"ip-10-0-2-1.ec2.internal", "ip-10-0-1-99.ec2.internal", false},
		{"api-7d9f", "api-7d10f", true},
		{"Node-2", "node-10", true}, // case-insensitive across the text runs
	}
	for _, c := range cases {
		if got := cellLess(c.a, c.b); got != c.want {
			t.Errorf("cellLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCellLessEmptySortsLast(t *testing.T) {
	// A real value should come before "-" / "<none>" / "" regardless of direction.
	for _, empty := range []string{"-", "", "<none>"} {
		if cellLess(empty, "5m") {
			t.Errorf("empty %q should not sort before a numeric value", empty)
		}
		if !cellLess("5m", empty) {
			t.Errorf("numeric value should sort before empty %q", empty)
		}
	}
}

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"a2", "a10", true},
		{"a10", "a2", false},
		{"a02", "a2", true}, // equal by value; byte order breaks the tie
		{"a2", "a02", false},
		{"a", "a1", true},   // prefix sorts first
		{"a1", "a-1", true}, // digits sort before punctuation
		{"abc", "abd", true},
		{"ABC", "abd", true},
		{"same", "same", false},
		{"x9y", "x9z", true},
		{"x9y", "x10a", true},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if c.a != c.b && naturalLess(c.a, c.b) == naturalLess(c.b, c.a) {
			t.Errorf("naturalLess(%q, %q) is not antisymmetric", c.a, c.b)
		}
	}
}
