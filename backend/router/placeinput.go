package router

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	// The place in a map link, not where its view is.
	linkedPlace = regexp.MustCompile(`!3d(-?\d+\.\d+)!4d(-?\d+\.\d+)`)
	coordinates = regexp.MustCompile(`(-?\d{1,2}\.\d+)(?:\s*,\s*|%2[cC]|\s+)(-?\d{1,3}\.\d+)`)
)

func (p *Places) pasted(query string) (Place, bool) {
	match := linkedPlace.FindStringSubmatch(query)
	if match == nil {
		match = coordinates.FindStringSubmatch(query)
	}
	if match == nil {
		return Place{}, false
	}
	lat, errLat := strconv.ParseFloat(match[1], 64)
	lon, errLon := strconv.ParseFloat(match[2], 64)
	if errLat != nil || errLon != nil || !p.box.contains(lat, lon) {
		return Place{}, false
	}
	label := fmt.Sprintf("%.5f, %.5f", lat, lon)
	name := label
	if near, ok := p.Nearby(lat, lon); ok {
		label, name = near.Label, near.Name
	}
	return Place{ID: "pasted", Lat: lat, Lon: lon, Label: label, Name: name}, true
}

const (
	oneEditLetters  = 4
	twoEditsLetters = 8
)

func allowedEdits(letters int) int {
	switch {
	case letters >= twoEditsLetters:
		return 2
	case letters >= oneEditLetters:
		return 1
	}
	return 0
}

func (p *Places) respelled(tokens []string) []string {
	out := make([]string, len(tokens))
	for i, t := range tokens {
		out[i] = t
		// 24 is not 25.
		if !strings.ContainsFunc(t, unicode.IsDigit) && len(p.find([]string{t})) == 0 {
			out[i] = p.closestWord(t)
		}
	}
	return out
}

func (p *Places) closestWord(token string) string {
	t := []rune(token)
	allowed := allowedEdits(len(t))
	best, bestEdits, bestCount := token, allowed+1, int32(0)
	for word, count := range p.wordCounts {
		w := []rune(word)
		if len(w) < len(t)-allowed {
			continue
		}
		// The query may stop mid-word.
		edits := min(editDistance(t, w, allowed), editDistance(t, w[:min(len(w), len(t))], allowed))
		if edits < bestEdits || edits == bestEdits && edits <= allowed && (count > bestCount || count == bestCount && word < best) {
			best, bestEdits, bestCount = word, edits, count
		}
	}
	return best
}

func editDistance(a, b []rune, limit int) int {
	if len(a)-len(b) > limit || len(b)-len(a) > limit {
		return limit + 1
	}
	before, previous := make([]int, len(b)+1), make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		lowest := i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				current[j] = min(current[j], before[j-2]+1)
			}
			lowest = min(lowest, current[j])
		}
		if lowest > limit {
			return limit + 1
		}
		before, previous, current = previous, current, before
	}
	return previous[len(b)]
}
