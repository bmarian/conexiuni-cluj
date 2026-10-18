package router

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type placeKind uint8

const (
	placeAddress placeKind = iota
	placeNamed
	placeStreet
	placeDistrict
	placeTown
)

type placeTags struct {
	kind                         placeKind
	name, street, number, suburb string
	// A city or a town, not a village: what a district belongs to.
	big bool
}

// pbfPlace is a tagged element the reader kept: a node with its position, a way by its first node.
type pbfPlace struct {
	placeTags
	node     int64
	lat, lon float64
	placed   bool
}

var namedKeys = []string{"amenity", "shop", "tourism", "leisure", "office", "craft", "healthcare", "historic", "railway", "aeroway", "building"}

func placeOf(tag func(string) string, way bool) (placeTags, bool) {
	name := strings.TrimSpace(tag("name"))
	switch p := tag("place"); p {
	case "city", "town", "village":
		return placeTags{kind: placeTown, name: name, big: p != "village"}, name != ""
	case "suburb", "neighbourhood", "quarter":
		return placeTags{kind: placeDistrict, name: name}, name != ""
	}
	street, number := strings.TrimSpace(tag("addr:street")), strings.TrimSpace(tag("addr:housenumber"))
	suburb := strings.TrimSpace(tag("addr:suburb"))
	if street != "" && number != "" {
		return placeTags{kind: placeAddress, name: name, street: street, number: number, suburb: suburb}, true
	}
	if name == "" {
		return placeTags{}, false
	}
	if tag("highway") != "" {
		return placeTags{kind: placeStreet, name: name}, way
	}
	for _, k := range namedKeys {
		if tag(k) != "" {
			return placeTags{kind: placeNamed, name: name, street: street, suburb: suburb}, true
		}
	}
	return placeTags{}, false
}

type Place struct {
	ID    string  `json:"id"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Label string  `json:"label"`
}

type placeEntry struct {
	kind     placeKind
	lat, lon float64
	label    string
	words    []string
	// How many of the words are the place's own, before the area it is in.
	own    int
	number string
}

type Places struct {
	entries []placeEntry
	// Entries by the first letters, up to three, of each of their words.
	index  map[string][]int32
	center Point
}

// How far a place still is the one at a position: the GPS error of a phone.
const nearbyReach = 250.0

func newPlaces(raw []pbfPlace, box BBox) *Places {
	p := &Places{index: make(map[string][]int32), center: Point{(box.MinLat + box.MaxLat) / 2, (box.MinLon + box.MaxLon) / 2}}
	var districts, towns, cities []pbfPlace
	for _, r := range raw {
		switch r.kind {
		case placeDistrict:
			districts = append(districts, r)
		case placeTown:
			towns = append(towns, r)
			if r.big {
				cities = append(cities, r)
			}
		}
	}
	if len(cities) == 0 {
		cities = towns
	}
	// The area a place is in: its district when one is nearer than any town, else the town.
	area := func(r pbfPlace) string {
		if r.suburb != "" {
			return r.suburb
		}
		d, dm := nearestPlace(districts, r.lat, r.lon)
		t, tm := nearestPlace(towns, r.lat, r.lon)
		if d != nil && (t == nil || dm < tm) {
			return d.name
		}
		if t != nil {
			return t.name
		}
		return ""
	}
	seen := make(map[string]bool)
	for _, r := range raw {
		var parts []string
		switch r.kind {
		case placeAddress:
			if r.name != "" && fold(r.name) != fold(r.street) {
				parts = append(parts, r.name)
			}
			parts = append(parts, r.street, r.number)
		case placeNamed:
			parts = append(parts, r.name)
			if r.street != "" {
				parts = append(parts, r.street)
			}
		case placeStreet:
			parts = append(parts, r.name)
		case placeDistrict, placeTown:
			parts = append(parts, r.name)
		}
		own := len(words(strings.Join(parts, ", ")))
		switch {
		case r.kind == placeDistrict:
			if t, _ := nearestPlace(cities, r.lat, r.lon); t != nil {
				parts = append(parts, t.name)
			}
		case r.kind < placeDistrict:
			if a := area(r); a != "" {
				parts = append(parts, a)
			}
		}
		label := strings.Join(parts, ", ")
		key := strconv.Itoa(int(r.kind)) + "|" + fold(label)
		if seen[key] {
			continue
		}
		seen[key] = true
		p.add(r.kind, r.lat, r.lon, label, own, r.number)
	}
	return p
}

func nearestPlace(among []pbfPlace, lat, lon float64) (*pbfPlace, float64) {
	var best *pbfPlace
	bestM := 0.0
	for i := range among {
		if m := haversine(lat, lon, among[i].lat, among[i].lon); best == nil || m < bestM {
			best, bestM = &among[i], m
		}
	}
	return best, bestM
}

func (p *Places) add(kind placeKind, lat, lon float64, label string, own int, number string) {
	id := int32(len(p.entries))
	e := placeEntry{kind: kind, lat: lat, lon: lon, label: label, words: words(label), own: own, number: fold(number)}
	p.entries = append(p.entries, e)
	keyed := make(map[string]bool)
	for _, w := range e.words {
		r := []rune(w)
		for n := 1; n <= min(3, len(r)); n++ {
			if k := string(r[:n]); !keyed[k] {
				keyed[k] = true
				p.index[k] = append(p.index[k], id)
			}
		}
	}
}

func (p *Places) Count() int { return len(p.entries) }

// Every query word must start a word of the place; best match first, then nearest the centre.
func (p *Places) Search(query string, limit int) []Place {
	tokens := words(query)
	if len(tokens) == 0 || limit <= 0 {
		return nil
	}
	// Candidates come from the word with the fewest places starting like it.
	var candidates []int32
	for i, t := range tokens {
		r := []rune(t)
		c := p.index[string(r[:min(3, len(r))])]
		if i == 0 || len(c) < len(candidates) {
			candidates = c
		}
	}
	type hit struct {
		id          int32
		score, dist float64
	}
	var hits []hit
	for _, id := range candidates {
		e := &p.entries[id]
		if score, ok := e.match(tokens); ok {
			hits = append(hits, hit{id, score, haversine(e.lat, e.lon, p.center.Lat, p.center.Lon)})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].dist < hits[j].dist
	})
	out := make([]Place, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, p.place(h.id))
	}
	return out
}

// 2 per whole word, 1 per prefix, +3 for the house number, +1 when all own words match, -0.1 per unmatched word.
func (e *placeEntry) match(tokens []string) (float64, bool) {
	score := 0.0
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
		if e.number != "" && t == e.number {
			score += 3
		}
	}
	whole := true
	for i, u := range used {
		if !u {
			score -= 0.1
			whole = whole && i >= e.own
		}
	}
	if whole {
		score++
	}
	return score, true
}

// Nearby is the place at a position: the closest address or named place in reach, else the closest street.
func (p *Places) Nearby(lat, lon float64) (Place, bool) {
	for _, kinds := range []func(placeKind) bool{
		func(k placeKind) bool { return k == placeAddress || k == placeNamed },
		func(k placeKind) bool { return k == placeStreet },
	} {
		best, bestM := int32(-1), nearbyReach
		for i := range p.entries {
			e := &p.entries[i]
			if m := haversine(lat, lon, e.lat, e.lon); kinds(e.kind) && m < bestM {
				best, bestM = int32(i), m
			}
		}
		if best >= 0 {
			return p.place(best), true
		}
	}
	return Place{}, false
}

func (p *Places) place(id int32) Place {
	e := &p.entries[id]
	return Place{ID: strconv.Itoa(int(id)), Lat: e.lat, Lon: e.lon, Label: e.label}
}

// A chained transformer keeps state, so every call gets its own: cities build their places at the same time.
func folder() transform.Transformer {
	return transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
}

// fold lowers text and drops its accents, so "Brâncuși" is found as "brancusi".
func fold(s string) string {
	out, _, err := transform.String(folder(), s)
	if err != nil {
		out = s
	}
	return strings.ToLower(out)
}

func words(s string) []string {
	return strings.FieldsFunc(fold(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
