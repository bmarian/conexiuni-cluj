package slugs

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"math"
	"sort"
	"strconv"
	"strings"
)

// By feeds.ModeRank.
var modeWords = [...]string{"metrou", "tren", "tramvai", "troleibuz", "autobuz"}

type item struct {
	id         int
	base       string
	mode       int
	precedence int
	at         *feeds.Point
	town       string
}

func baseOf(name string, id int) string {
	if s := router.Slug(name); s != "" {
		return s
	}
	return strconv.Itoa(id)
}

func followStations(snap *feeds.Snapshot, rows []row) ([]row, []row) {
	station := make(map[int]int, len(snap.Stops))
	for _, s := range snap.Stops {
		station[s.StopID] = s.Station()
	}
	out := make([]row, len(rows))
	var moved []row
	for i, r := range rows {
		if id, ok := station[r.id]; ok && r.kind == Station && id != r.id {
			r.id = id
			moved = append(moved, r)
		}
		out[i] = r
	}
	return out, moved
}

func stationItems(snap *feeds.Snapshot) []*item {
	stops := make(map[int]models.Stop, len(snap.Stops))
	for _, s := range snap.Stops {
		stops[s.StopID] = s
	}
	routeOf := make(map[string]int, len(snap.Trips))
	for _, t := range snap.Trips {
		routeOf[t.TripID] = t.RouteID
	}
	rank := make(map[int]int, len(snap.Routes))
	for _, r := range snap.Routes {
		rank[r.RouteID] = feeds.ModeRank(r.RouteType)
	}
	byID := map[int]*item{}
	lines := map[int]map[int]bool{}
	platforms := map[int][]models.Stop{}
	counted := map[int]bool{}
	var out []*item
	for _, ts := range snap.TripStops {
		stop, ok := stops[ts.StopID]
		if !ok {
			continue
		}
		id := stop.Station()
		it := byID[id]
		if it == nil {
			name := stop.StopName
			if own, ok := stops[id]; ok {
				name = own.StopName
			}
			it = &item{id: id, base: baseOf(name, id), mode: feeds.ModeRank(models.Bus)}
			byID[id], lines[id] = it, map[int]bool{}
			out = append(out, it)
		}
		if !counted[stop.StopID] {
			counted[stop.StopID] = true
			platforms[id] = append(platforms[id], stop)
		}
		route := routeOf[ts.TripID]
		lines[id][route] = true
		if r, ok := rank[route]; ok {
			it.mode = min(it.mode, r)
		}
	}
	for _, it := range out {
		it.precedence = len(lines[it.id])
		var at feeds.Point
		for _, p := range platforms[it.id] {
			at.Lat += p.StopLat / float64(len(platforms[it.id]))
			at.Lon += p.StopLon / float64(len(platforms[it.id]))
		}
		it.at = &at
	}
	return out
}

func lineItems(snap *feeds.Snapshot) []*item {
	out := make([]*item, 0, len(snap.Routes))
	for _, r := range snap.Routes {
		it := &item{id: r.RouteID, base: baseOf(r.RouteShortName, r.RouteID), mode: feeds.ModeRank(r.RouteType)}
		if strings.ToLower(strings.TrimSpace(r.RouteShortName)) == it.base {
			it.precedence = 1
		}
		out = append(out, it)
	}
	return out
}

func assign(kind Kind, items []*item, rows []row, locate feeds.Locate, logf func(string, ...any)) []row {
	owner := map[string]int{}
	kept := map[int]bool{}
	byID := make(map[int]*item, len(items))
	for _, it := range items {
		byID[it.id] = it
	}
	for _, r := range rows {
		if r.kind != kind {
			continue
		}
		owner[r.slug] = r.id
		if it := byID[r.id]; r.current && it != nil && it.base == r.base {
			kept[r.id] = true
		}
	}
	byBase := map[string][]*item{}
	var needing []*item
	for _, it := range items {
		byBase[it.base] = append(byBase[it.base], it)
		if !kept[it.id] {
			needing = append(needing, it)
		}
	}
	if len(needing) == 0 {
		return nil
	}
	sort.Slice(needing, func(i, j int) bool {
		a, b := needing[i], needing[j]
		if a.base != b.base {
			return a.base < b.base
		}
		if a.precedence != b.precedence {
			return a.precedence > b.precedence
		}
		return a.id < b.id
	})
	if locate != nil {
		if err := locateTowns(needing, byBase, locate); err != nil {
			logf("slugs: towns: %v", err)
		}
	}
	taken := func(slug string, id int) bool {
		owned, ok := owner[slug]
		return ok && owned != id
	}
	out := make([]row, 0, len(needing))
	for _, it := range needing {
		slug := it.base
		if taken(slug, it.id) {
			stem := strings.Join(append([]string{it.base}, qualifiers(it, byBase[it.base])...), "-")
			slug = stem
			for n := 2; taken(slug, it.id); n++ {
				slug = stem + "-" + strconv.Itoa(n)
			}
		}
		owner[slug] = it.id
		out = append(out, row{kind: kind, slug: slug, id: it.id, base: it.base, current: true})
	}
	return out
}

func locateTowns(needing []*item, byBase map[string][]*item, locate feeds.Locate) error {
	var points []feeds.Point
	var at []*item
	seen := map[string]bool{}
	for _, it := range needing {
		if seen[it.base] || len(byBase[it.base]) < 2 {
			continue
		}
		seen[it.base] = true
		for _, o := range byBase[it.base] {
			if o.at != nil {
				points, at = append(points, *o.at), append(at, o)
			}
		}
	}
	if len(points) == 0 {
		return nil
	}
	towns, _, err := locate(points)
	if err != nil {
		return err
	}
	for i, o := range at {
		o.town = towns[i]
	}
	return nil
}

func qualifiers(it *item, namesakes []*item) []string {
	var parts []string
	group := namesakes
	if same, differs := narrow(group, it, func(o *item) string { return o.town }); differs && it.town != "" {
		parts, group = append(parts, router.Slug(it.town)), same
	}
	if same, differs := narrow(group, it, func(o *item) string { return modeWords[o.mode] }); differs {
		parts, group = append(parts, modeWords[it.mode]), same
	}
	if len(group) > 1 && it.at != nil {
		parts = append(parts, side(it, group))
	}
	return parts
}

func narrow(group []*item, it *item, key func(*item) string) ([]*item, bool) {
	var same []*item
	for _, o := range group {
		if key(o) == key(it) {
			same = append(same, o)
		}
	}
	return same, len(same) < len(group)
}

func side(it *item, group []*item) string {
	var lat, lon float64
	for _, o := range group {
		lat += o.at.Lat / float64(len(group))
		lon += o.at.Lon / float64(len(group))
	}
	north := func(o *item) float64 { return math.Copysign(geo.Haversine(lat, lon, o.at.Lat, lon), o.at.Lat-lat) }
	east := func(o *item) float64 { return math.Copysign(geo.Haversine(lat, lon, lat, o.at.Lon), o.at.Lon-lon) }
	if spread(group, north) >= spread(group, east) {
		if north(it) > 0 {
			return "nord"
		}
		return "sud"
	}
	if east(it) > 0 {
		return "est"
	}
	return "vest"
}

func spread(group []*item, along func(*item) float64) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, o := range group {
		lo, hi = math.Min(lo, along(o)), math.Max(hi, along(o))
	}
	return hi - lo
}
