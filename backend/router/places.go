package router

import (
	"slices"
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
	aliases                      string
	// A city or a town, not a village: what a district belongs to.
	big     bool
	notable bool
	// Only for nodes: a way stands on its first node.
	place string
}

// pbfPlace is a tagged element the reader kept: a node with its position, a way by its first node.
type pbfPlace struct {
	placeTags
	node     int64
	lat, lon float64
	placed   bool
}

var (
	namedKeys = []string{
		"amenity", "shop", "tourism", "leisure", "office", "craft", "healthcare", "historic", "railway", "aeroway", "building", "landuse", "natural",
	}
	aliasKeys   = []string{"alt_name", "old_name", "short_name", "official_name", "loc_name"}
	notableKeys = []string{"wikidata", "wikipedia"}
)

func placeOf(tag func(string) string, way bool) (placeTags, bool) {
	p, ok := placeKindOf(tag, way)
	if !ok {
		return p, false
	}
	var aliases []string
	for _, k := range aliasKeys {
		aliases = append(aliases, tag(k))
	}
	p.aliases = strings.Join(aliases, " ")
	p.notable = slices.ContainsFunc(notableKeys, func(k string) bool { return tag(k) != "" })
	return p, true
}

func placeKindOf(tag func(string) string, way bool) (placeTags, bool) {
	name := strings.TrimSpace(tag("name"))
	p := tag("place")
	node := p
	if way {
		node = ""
	}
	switch p {
	case "city", "town", "village":
		return placeTags{kind: placeTown, name: name, big: p != "village", place: node}, name != ""
	case "suburb", "neighbourhood", "quarter":
		return placeTags{kind: placeDistrict, name: name, place: node}, name != ""
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
	// The start of Label.
	Name string `json:"name"`
}

type placeEntry struct {
	kind     placeKind
	lat, lon float64
	label    string
	words    []string
	// How many of the words are the place's own name, before its address, area and aliases.
	own     int
	number  string
	notable bool
	nameLen int
}

type Places struct {
	entries []placeEntry
	// Entries by the first letters, up to three, of each of their words.
	index      map[string][]int32
	center     Point
	areas      []Area
	townWords  map[string]bool
	wordCounts map[string]int32
	box        BBox
}

type Area struct {
	Name  string  `json:"name"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Place string  `json:"place"`
}

// How far a place still is the one at a position: the GPS error of a phone.
const nearbyReach = 250.0

func newPlaces(raw []pbfPlace, box BBox) *Places {
	p := &Places{
		index:      make(map[string][]int32),
		center:     Point{(box.MinLat + box.MaxLat) / 2, (box.MinLon + box.MaxLon) / 2},
		townWords:  make(map[string]bool),
		wordCounts: make(map[string]int32),
		box:        box,
	}
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
			for _, w := range words(r.name) {
				p.townWords[w] = true
			}
		}
	}
	if len(cities) == 0 {
		cities = towns
	}
	p.areas = areasOf(append(towns, districts...))
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
		name := r.name
		switch r.kind {
		case placeAddress:
			if r.name != "" && fold(r.name) != fold(r.street) {
				parts = append(parts, r.name)
			} else {
				name = r.street + ", " + r.number
			}
			parts = append(parts, r.street, r.number)
		case placeNamed:
			parts = append(parts, r.name)
			if r.street != "" {
				parts = append(parts, r.street)
			}
		case placeStreet, placeDistrict, placeTown:
			parts = append(parts, r.name)
		}
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
		p.add(r, label, name)
	}
	return p
}

func areasOf(places []pbfPlace) []Area {
	out := []Area{}
	seen := make(map[string]bool)
	for _, r := range places {
		key := r.place + "|" + fold(r.name)
		if r.place == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Area{Name: r.name, Lat: r.lat, Lon: r.lon, Place: r.place})
	}
	return out
}

func (p *Places) Areas() []Area { return p.areas }

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

func (p *Places) add(r pbfPlace, label, name string) {
	id := int32(len(p.entries))
	e := placeEntry{
		kind: r.kind, lat: r.lat, lon: r.lon, label: label, number: fold(r.number), notable: r.notable,
		words: append(words(label), words(r.aliases)...), own: len(words(name)), nameLen: len(name),
	}
	p.entries = append(p.entries, e)
	keyed := make(map[string]bool)
	for _, w := range e.words {
		p.wordCounts[w]++
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
	return Place{ID: strconv.Itoa(int(id)), Lat: e.lat, Lon: e.lon, Label: e.label, Name: e.label[:e.nameLen]}
}

// A Chain keeps state, and cities build their places concurrently.
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

func Slug(name string) string { return strings.Join(words(name), "-") }

func words(s string) []string {
	return strings.FieldsFunc(fold(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
