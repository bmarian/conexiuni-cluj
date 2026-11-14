package search

import (
	"strings"
	"unicode"

	"conexiuni-cluj/router"
)

// Port of frontend/src/utils/stopSearch.ts.

const wordsRank = 3

// 0 the name starts with the query, 1 the own name has it, 2 only the "(Town)" part does, -1 no match.
func matchRank(name, query string) int {
	at := strings.Index(name, query)
	switch {
	case at < 0:
		return -1
	case at == 0:
		return 0
	}
	own := strings.Index(name, " (")
	if own < 0 || at < own {
		return 1
	}
	return 2
}

type nameWord struct {
	word  string
	start int
	short bool
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func nameWords(name string) []nameWord {
	var out []nameWord
	runes := []rune(name)
	for i := 0; i < len(runes); {
		if !isWordRune(runes[i]) {
			i++
			continue
		}
		j := i
		for j < len(runes) && isWordRune(runes[j]) {
			j++
		}
		out = append(out, nameWord{word: string(runes[i:j]), start: i, short: j < len(runes) && runes[j] == '.'})
		i = j
	}
	return out
}

func allLetters(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) })
}

// "P-ta" and "C-tin": one or two letters, a hyphen, two or three letters.
func contractions(name string, found []nameWord) [][2]string {
	runes := []rune(name)
	var out [][2]string
	for i := 0; i+1 < len(found); i++ {
		a, b := found[i], found[i+1]
		end := a.start + len([]rune(a.word))
		la, lb := len([]rune(a.word)), len([]rune(b.word))
		if end < len(runes) && runes[end] == '-' && b.start == end+1 && la <= 2 && lb >= 2 && lb <= 3 && allLetters(a.word) && allLetters(b.word) {
			out = append(out, [2]string{a.word, b.word})
		}
	}
	return out
}

func queryWords(query string) []string {
	query = strings.NewReplacer("b-dul", "bulevardul", "p-ta", "piata").Replace(query)
	var all, naming []string
	for _, w := range nameWords(query) {
		word := w.word
		if full, ok := router.StreetKind(word); ok {
			word = full
		}
		all = append(all, word)
		if _, kind := router.StreetKind(word); !kind {
			naming = append(naming, word)
		}
	}
	if len(naming) > 0 {
		return naming
	}
	return all
}

// "M." stands for "mihai" only beside a word matched in full, or "marasti" would find it.
func nameMatches(name, query string, typos bool) bool {
	words := queryWords(query)
	if len(words) == 0 {
		return false
	}
	found := nameWords(name)
	contracted := contractions(name, found)
	whole := false
	for _, word := range words {
		full := false
		for _, f := range found {
			if strings.HasPrefix(f.word, word) || typos && !f.short && router.Misspells(word, f.word) {
				full = true
				break
			}
		}
		for _, c := range contracted {
			full = full || strings.HasPrefix(word, c[0]) && strings.HasSuffix(word, c[1])
		}
		if full {
			whole = true
			continue
		}
		short := false
		for _, f := range found {
			short = short || f.short && strings.HasPrefix(word, f.word)
		}
		if !short {
			return false
		}
	}
	return whole
}

type ranked[T any] struct {
	item T
	rank int
}

// Names and query folded.
func matching[T any](items []T, nameOf func(T) string, query string) []ranked[T] {
	var exact []ranked[T]
	for _, item := range items {
		name := nameOf(item)
		rank := matchRank(name, query)
		if rank < 0 && nameMatches(name, query, false) {
			rank = wordsRank
		}
		if rank >= 0 {
			exact = append(exact, ranked[T]{item, rank})
		}
	}
	if len(exact) > 0 {
		return exact
	}
	var spelled []ranked[T]
	for _, item := range items {
		if nameMatches(nameOf(item), query, true) {
			spelled = append(spelled, ranked[T]{item, wordsRank})
		}
	}
	return spelled
}
