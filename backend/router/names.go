package router

import "unicode"

func Fold(s string) string { return fold(s) }

func Words(s string) []string { return words(s) }

func StreetKind(w string) (string, bool) {
	full, ok := streetKinds[w]
	return full, ok
}

func Misspells(word, nameWord string) bool {
	w, n := []rune(word), []rune(nameWord)
	allowed := allowedEdits(len(w))
	for _, r := range w {
		if unicode.IsDigit(r) {
			return false
		}
	}
	if allowed == 0 {
		return false
	}
	return editDistance(w, n, allowed) <= allowed || editDistance(w, n[:min(len(n), len(w))], allowed) <= allowed
}
