package router

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var streetKinds = map[string]string{
	"strada": "strada", "str": "strada",
	"bulevardul": "bulevardul", "bulevard": "bulevardul", "bd": "bulevardul", "bld": "bulevardul", "blvd": "bulevardul", "bdul": "bulevardul",
	"calea": "calea", "cal": "calea",
	"aleea": "aleea", "al": "aleea",
	"piata": "piata", "pta": "piata",
	"soseaua": "soseaua", "sos": "soseaua",
	"splaiul": "splaiul", "spl": "splaiul",
	"intrarea": "intrarea", "intr": "intrarea",
	"pasajul": "pasajul",
	"drumul":  "drumul",
}

var hyphenated = strings.NewReplacer("b-dul", "bulevardul", "p-ta", "piata")

var addressWords = map[string]bool{
	"nr": true, "numar": true, "numarul": true, "romania": true, "jud": true, "judet": true, "judetul": true, "mun": true, "municipiul": true,
}

const postalCodeDigits = 6

type placeHit struct {
	id          int32
	score, dist float64
	numberGap   float64
}

// Every query word must start a word of the place, best match first, then nearest the centre.
func (p *Places) Search(query string, limit int) []Place {
	if limit <= 0 {
		return nil
	}
	if place, ok := p.pasted(query); ok {
		return []Place{place}
	}
	tokens := queryWords(query)
	if len(tokens) == 0 {
		return nil
	}
	hits := p.lookup(tokens)
	if len(hits) == 0 {
		if spelled := p.respelled(tokens); !slices.Equal(spelled, tokens) {
			hits = p.lookup(spelled)
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		areaA, areaB := p.entries[a.id].kind >= placeDistrict, p.entries[b.id].kind >= placeDistrict
		switch {
		case a.numberGap != b.numberGap:
			return a.numberGap < b.numberGap
		case a.score != b.score:
			return a.score > b.score
		case areaA != areaB:
			return areaA
		}
		return a.dist < b.dist
	})
	out := make([]Place, 0, min(limit, len(hits)))
	var kept []int32
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		if !p.sameAsAny(h.id, kept) {
			kept = append(kept, h.id)
			out = append(out, p.place(h.id))
		}
	}
	return out
}

func (p *Places) lookup(tokens []string) []placeHit {
	hits := p.find(tokens)
	if len(hits) > 0 {
		return hits
	}
	naming, number := p.naming(tokens)
	if len(naming) == 0 || len(naming) == len(tokens) {
		return nil
	}
	hits = p.find(naming)
	if number != "" {
		p.byNumber(hits, number)
	}
	return hits
}

func (p *Places) find(tokens []string) []placeHit {
	// Candidates come from the word with the fewest places starting like it.
	var candidates []int32
	for i, t := range tokens {
		r := []rune(t)
		c := p.index[string(r[:min(3, len(r))])]
		if i == 0 || len(c) < len(candidates) {
			candidates = c
		}
	}
	var hits []placeHit
	for _, id := range candidates {
		e := &p.entries[id]
		if score, ok := p.match(e, tokens); ok {
			hits = append(hits, placeHit{id: id, score: score, dist: haversine(e.lat, e.lon, p.center.Lat, p.center.Lon)})
		}
	}
	return hits
}

func (p *Places) naming(tokens []string) ([]string, string) {
	var naming []string
	number := ""
	inNumber := false
	for _, t := range tokens {
		digit := unicode.IsDigit([]rune(t)[0])
		if digit && !inNumber {
			number = t
		}
		// "14-16" is two words, one number.
		inNumber = digit
		if !digit && !p.generic(t) {
			naming = append(naming, t)
		}
	}
	return naming, number
}

func (p *Places) byNumber(hits []placeHit, number string) {
	asked := houseNumber(number)
	for i := range hits {
		hits[i].numberGap = math.Inf(1)
		if n := houseNumber(p.entries[hits[i].id].number); n >= 0 {
			hits[i].numberGap = math.Abs(n - asked)
		}
	}
}

func notDigit(r rune) bool { return !unicode.IsDigit(r) }

func houseNumber(s string) float64 {
	end := strings.IndexFunc(s, notDigit)
	if end < 0 {
		end = len(s)
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return -1
	}
	return float64(n)
}

// Mapped twice, or split by a district's edge.
func (p *Places) sameAsAny(id int32, kept []int32) bool {
	e := &p.entries[id]
	return slices.ContainsFunc(kept, func(k int32) bool {
		o := &p.entries[k]
		return slices.Equal(e.words[:e.own], o.words[:o.own]) && haversine(e.lat, e.lon, o.lat, o.lon) < nearbyReach
	})
}

func (p *Places) generic(w string) bool {
	_, street := streetKinds[w]
	return street || p.townWords[w]
}

func queryWords(query string) []string {
	var out []string
	for _, w := range words(hyphenated.Replace(fold(query))) {
		if addressWords[w] || isPostalCode(w) {
			continue
		}
		if full, ok := streetKinds[w]; ok {
			w = full
		}
		out = append(out, w)
	}
	return out
}

func isPostalCode(w string) bool {
	return len(w) == postalCodeDigits && !strings.ContainsFunc(w, notDigit)
}

// 2 per whole word, 1 per prefix, +3 the house number, +1 all of its name, -0.1 per word of it left; +2 notable by name, -1 not by name.
func (p *Places) match(e *placeEntry, tokens []string) (float64, bool) {
	score := 0.0
	whole := true
	used := make([]bool, len(e.words))
	for _, t := range tokens {
		best, at := 0.0, -1
		for i, w := range e.words {
			switch {
			case w == t:
				if best < 2 {
					best, at = 2, i
				}
			case strings.HasPrefix(w, t):
				if best < 1 {
					best, at = 1, i
				}
			}
		}
		if at < 0 {
			return 0, false
		}
		used[at] = true
		score += best
		whole = whole && best == 2
		if e.number != "" && t == e.number {
			score += 3
		}
	}
	byName := false
	for i, w := range e.words[:e.own] {
		byName = byName || used[i]
		if !used[i] && !p.generic(w) {
			score -= 0.1
			whole = false
		}
	}
	if whole {
		score++
	}
	switch {
	case !byName:
		score--
	case e.notable:
		score += 2
	}
	return score, true
}
