package ui

import (
	"sort"
	"strings"
)

// fuzzyRank filters items to those whose hay(item) matches q, ordered by score
// (best first, stable). An empty q keeps every item in its original order.
//
// Matching is tiered: when at least one item contains q as a contiguous
// substring, only substring matches are kept. Subsequence matching is the
// fallback for abbreviations, so typing part of a name does not drown the
// real hits in scattered-letter matches.
func fuzzyRank[T any](items []T, q string, hay func(T) string) []T {
	type scored struct {
		item  T
		score int
		exact bool
	}
	var ms []scored
	anyExact := false
	for _, it := range items {
		if s, exact, ok := matchScore(q, hay(it)); ok {
			ms = append(ms, scored{it, s, exact})
			anyExact = anyExact || exact
		}
	}
	if anyExact {
		kept := ms[:0]
		for _, m := range ms {
			if m.exact {
				kept = append(kept, m)
			}
		}
		ms = kept
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].score > ms[j].score })
	out := make([]T, len(ms))
	for i, m := range ms {
		out[i] = m.item
	}
	return out
}

// matchScore scores text against pattern. exact reports a contiguous,
// case-insensitive substring match; otherwise the score comes from
// fuzzyScore. An empty pattern matches everything.
func matchScore(pattern, text string) (score int, exact, ok bool) {
	if pattern == "" {
		return 0, true, true
	}
	if s, ok := substringScore(pattern, text); ok {
		return s, true, true
	}
	s, ok := fuzzyScore(pattern, text)
	return s, false, ok
}

// substringScore reports whether pattern occurs in text as a substring. The
// score prefers a hit at the start of text, then at a word boundary, then an
// earlier position, then a shorter text.
func substringScore(pattern, text string) (int, bool) {
	p := strings.ToLower(pattern)
	t := strings.ToLower(text)
	first := strings.Index(t, p)
	if first < 0 {
		return 0, false
	}
	best := first
	for i := first; i >= 0 && i < len(t); {
		if i == 0 || isBoundary(t[i-1]) {
			best = i
			break
		}
		next := strings.Index(t[i+1:], p)
		if next < 0 {
			break
		}
		i += 1 + next
	}
	score := 1000
	switch {
	case best == 0:
		score += 200
	case isBoundary(t[best-1]):
		score += 100
	}
	score -= best
	score -= len(t) / 12
	return score, true
}

// fuzzyScore reports whether pattern matches text as a subsequence, with a
// score that rewards consecutive runs and word-boundary hits so the most
// relevant matches sort first. An empty pattern matches everything.
func fuzzyScore(pattern, text string) (int, bool) {
	if pattern == "" {
		return 0, true
	}
	p := strings.ToLower(pattern)
	t := strings.ToLower(text)

	score := 0
	ti := 0
	last := -2
	for pi := 0; pi < len(p); pi++ {
		c := p[pi]
		found := false
		for ; ti < len(t); ti++ {
			if t[ti] != c {
				continue
			}
			score++
			if ti == last+1 {
				score += 6 // consecutive
			}
			if ti == 0 || isBoundary(t[ti-1]) {
				score += 10 // word boundary
			}
			last = ti
			ti++
			found = true
			break
		}
		if !found {
			return 0, false
		}
	}
	// Prefer shorter, tighter matches.
	score -= len(t) / 12
	return score, true
}

func isBoundary(b byte) bool {
	switch b {
	case ' ', '-', '_', '.', '/', ':':
		return true
	}
	return false
}
