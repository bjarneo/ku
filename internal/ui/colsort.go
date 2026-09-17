package ui

import (
	"strconv"
	"strings"
)

// cellLess orders two table cells. Duration-shaped values ("2m59s", "5d")
// compare by elapsed time, numeric-looking values (CPU "245m", MEM "1234Mi",
// percentages, restart counts, ready ratios) compare numerically, and
// everything else compares naturally: case-insensitive, with embedded digit
// runs compared by value so "node-2" sorts before "node-10". Unparseable/empty
// cells sort after numeric ones.
func cellLess(a, b string) bool {
	av, aok := sortVal(a)
	bv, bok := sortVal(b)
	if aok && bok {
		if av != bv {
			return av < bv
		}
		return false
	}
	if aok != bok {
		return aok
	}
	return naturalLess(a, b)
}

// naturalLess compares two strings chunk by chunk, where a chunk is either a
// run of ASCII digits or a run of anything else. Digit runs compare by numeric
// value (leading zeros ignored), other runs compare case-insensitively. When
// the two strings are equal under those rules, plain byte order breaks the tie
// so the result stays a strict weak ordering.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ad, bd := isDigit(a[i]), isDigit(b[j])
		if ad && bd {
			ai, bj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			an := strings.TrimLeft(a[ai:i], "0")
			bn := strings.TrimLeft(b[bj:j], "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			continue
		}
		if ad != bd {
			// Digits sort before letters and punctuation, so "node1" precedes
			// "node-a" regardless of the two runs' byte values.
			return ad
		}
		ac, bc := lower(a[i]), lower(b[j])
		if ac != bc {
			return ac < bc
		}
		i++
		j++
	}
	if len(a)-i != len(b)-j {
		return len(a)-i < len(b)-j
	}
	return a < b
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func sortVal(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "<none>" {
		return 0, false
	}
	// Duration cells appear under many column names (AGE, LAST SEEN, DURATION,
	// LAST SCHEDULE), so the value's shape decides, not the column name.
	if v, ok := parseAge(s); ok {
		return v, true
	}
	// CPU/MEM/percent/restarts/ready all lead with their number, so the leading
	// numeric prefix is a stable comparison key for them.
	return leadingNum(s)
}

// parseAge converts a Kubernetes age like "5d", "2y64d", or "1h30m" to seconds.
func parseAge(s string) (float64, bool) {
	units := map[byte]float64{'s': 1, 'm': 60, 'h': 3600, 'd': 86400, 'y': 31536000}
	var total float64
	var num strings.Builder
	matched := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			num.WriteByte(c)
			continue
		}
		mult, ok := units[c]
		if !ok || num.Len() == 0 {
			return 0, false
		}
		n, _ := strconv.Atoi(num.String())
		total += float64(n) * mult
		num.Reset()
		matched = true
	}
	if num.Len() > 0 { // trailing digits with no unit: not an age
		return 0, false
	}
	return total, matched
}

// leadingNum parses the leading numeric run of s (e.g. "245m" -> 245, "1/2" -> 1).
func leadingNum(s string) (float64, bool) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, false
	}
	f, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
