package suceava

import (
	"conexiuni-cluj/feeds"
	"slices"
	"strings"
	"unicode"
)

// A word of a stop name, folded; abbrev when written with a full stop ("Str.", "Sf.").
type word struct {
	text   string
	abbrev bool
}

func nameWords(name string) []word {
	var out []word
	for _, token := range strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' }) {
		parts := strings.Split(token, ".")
		for i, part := range parts {
			if text := feeds.NormalizeStopLabel(part); text != "" {
				out = append(out, word{text: text, abbrev: i < len(parts)-1})
			}
		}
	}
	return out
}

// The name as written and with its numbers spelled out, for searches that match words as typed.
func spellings(name string) []string {
	out := []string{name}
	var spelled []string
	changed := false
	for _, w := range strings.Fields(name) {
		if words := numerals[w]; len(words) > 0 {
			w, changed = words[0], true
		}
		spelled = append(spelled, w)
	}
	if changed {
		out = append(out, strings.Join(spelled, " "))
	}
	return out
}

// How many of a's words are words of b.
func wordsShared(a, b []word) int {
	n := 0
	for _, w := range a {
		if slices.ContainsFunc(b, func(v word) bool { return sameWord(w, v) }) {
			n++
		}
	}
	return n
}

// Romanian article endings: "Spital" and "Spitalul" are one stop.
var articleEndings = []string{"ul", "lui", "le", "a", "ua", "i", "ii", "ei", "lor", "ilor", "elor"}

// Romanian numerals, folded: "3 Movile" is the village "Trei Movile".
var numerals = map[string][]string{"1": {"unu", "una"}, "2": {"doi", "doua"}, "3": {"trei"}, "4": {"patru"}, "5": {"cinci"}, "6": {"sase"}, "7": {"sapte"}, "8": {"opt"}, "9": {"noua"}, "10": {"zece"}}

// The same word, the number it spells, its abbreviation of two letters or more (an initial stands for nothing), or the word with its article.
func sameWord(a, b word) bool {
	if a.text == b.text || slices.Contains(numerals[a.text], b.text) || slices.Contains(numerals[b.text], a.text) {
		return true
	}
	short, long := a, b
	if len(short.text) > len(long.text) {
		short, long = long, short
	}
	rest, ok := strings.CutPrefix(long.text, short.text)
	if !ok || strings.IndexFunc(short.text, unicode.IsDigit) >= 0 {
		return false
	}
	return (short.abbrev && len([]rune(short.text)) > 1) || slices.Contains(articleEndings, rest)
}
