package handlers

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// normalizeStopLabelForMatch mirrors the frontend's normalization:
// NFD decompose, drop combining marks, lowercase ASCII, collapse non-alphanumerics to single spaces.
// "Disp. Clăbucet" and "P-ța Alverna" → "disp clabucet" / "p ta alverna".
func normalizeStopLabelForMatch(label string) string {
	decomposed := norm.NFD.String(label)
	var b strings.Builder
	b.Grow(len(decomposed))
	prevSpace := true
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevSpace = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
			prevSpace = false
		default:
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// stopMatchScore ranks how closely two stop labels refer to the same place.
// 3 exact, 2 substring containment, 1 Levenshtein ≤ 2, 0 otherwise.
func stopMatchScore(a, b string) int {
	na := normalizeStopLabelForMatch(a)
	nb := normalizeStopLabelForMatch(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 3
	}
	if strings.Contains(na, nb) || strings.Contains(nb, na) {
		return 2
	}
	if levenshtein(na, nb) <= 2 {
		return 1
	}
	return 0
}

func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
