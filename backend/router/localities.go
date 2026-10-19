package router

import (
	"bufio"
	"io"
	"math"
	"slices"
	"strconv"
)

// Romania's OSM admin levels; Bucharest has no 8.
const (
	adminTown   = 8
	adminCounty = 4
)

var busRoads = map[string]bool{
	"motorway": true, "motorway_link": true, "trunk": true, "trunk_link": true,
	"primary": true, "primary_link": true, "secondary": true, "secondary_link": true,
	"tertiary": true, "tertiary_link": true, "unclassified": true, "residential": true,
	"living_street": true, "service": true, "road": true, "busway": true,
}

type boundary struct {
	pbfRelation
	box BBox
	ends []Point
}

func (b *boundary) contains(p Point) bool {
	if !b.box.contains(p.Lat, p.Lon) {
		return false
	}
	in := false
	for i := 0; i+1 < len(b.ends); i += 2 {
		a, c := b.ends[i], b.ends[i+1]
		if (a.Lat > p.Lat) != (c.Lat > p.Lat) && p.Lon < a.Lon+(p.Lat-a.Lat)/(c.Lat-a.Lat)*(c.Lon-a.Lon) {
			in = !in
		}
	}
	return in
}

func townOf(tag func(string) string, way bool) (placeTags, bool) {
	switch tag("place") {
	case "city", "town", "village":
		name := tag("name")
		return placeTags{kind: placeTown, name: name}, !way && name != ""
	}
	return placeTags{}, false
}

func adminBoundary(tag func(string) string) (pbfRelation, bool) {
	if tag("boundary") != "administrative" {
		return pbfRelation{}, false
	}
	level, _ := strconv.Atoi(tag("admin_level"))
	return pbfRelation{name: tag("name"), level: level}, level == adminTown || level == adminCounty
}

func Localities(r io.ReadSeeker, points []Point) (towns, streets []string, err error) {
	if len(points) == 0 {
		return nil, nil, nil
	}
	box := BBox{MinLat: math.Inf(1), MinLon: math.Inf(1), MaxLat: math.Inf(-1), MaxLon: math.Inf(-1)}
	for _, p := range points {
		box.MinLat, box.MaxLat = math.Min(box.MinLat, p.Lat), math.Max(box.MaxLat, p.Lat)
		box.MinLon, box.MaxLon = math.Min(box.MinLon, p.Lon), math.Max(box.MaxLon, p.Lon)
	}

	roads := &segments{grid: make(map[[2]int32][]int32), box: box}
	var labels []string
	index := make(map[int64]int32)
	var places []pbfPlace
	var relations []pbfRelation
	err = read(r, pbfFilter{
		node: func(_ int64, lat, lon float64) (bool, bool) { return box.contains(lat, lon), true },
		way: func(_ int64, tag func(string) string) (string, bool) {
			if !busRoads[tag("highway")] {
				return "", false
			}
			label := tag("name")
			if label == "" {
				label = tag("ref")
			}
			return label, label != ""
		},
		place:    townOf,
		relation: adminBoundary,
	}, func(b pbfBlock) {
		for _, n := range b.nodes {
			index[n.id] = int32(len(roads.lat))
			roads.lat, roads.lon = append(roads.lat, n.lat), append(roads.lon, n.lon)
		}
		for _, w := range b.ways {
			for i := 1; i < len(w.refs); i++ {
				u, okU := index[w.refs[i-1]]
				v, okV := index[w.refs[i]]
				if okU && okV && u != v {
					roads.addSegment(u, v)
					labels = append(labels, w.label)
				}
			}
		}
		places = append(places, b.places...)
		relations = append(relations, b.relations...)
	})
	if err != nil {
		return nil, nil, err
	}
	index = nil

	wanted := make(map[int64]bool)
	for _, rel := range relations {
		for _, w := range rel.ways {
			wanted[w] = true
		}
	}
	wayRefs := make(map[int64][]int64, len(wanted))
	err = read(r, pbfFilter{
		node: func(int64, float64, float64) (bool, bool) { return false, false },
		way: func(id int64, _ func(string) string) (string, bool) {
			return "", wanted[id]
		},
	}, func(b pbfBlock) {
		for _, w := range b.ways {
			wayRefs[w.id] = w.refs
		}
	})
	if err != nil {
		return nil, nil, err
	}

	var needed []int64
	for _, refs := range wayRefs {
		needed = append(needed, refs...)
	}
	slices.Sort(needed)
	needed = slices.Compact(needed)
	at, found := make([]Point, len(needed)), make([]bool, len(needed))
	err = read(r, pbfFilter{
		node: func(id int64, _, _ float64) (bool, bool) {
			_, found := slices.BinarySearch(needed, id)
			return found, false
		},
		way: func(int64, func(string) string) (string, bool) { return "", false },
	}, func(b pbfBlock) {
		for _, n := range b.nodes {
			i, _ := slices.BinarySearch(needed, n.id)
			at[i], found[i] = Point{n.lat, n.lon}, true
		}
	})
	if err != nil {
		return nil, nil, err
	}
	nodeAt := func(id int64) (Point, bool) {
		i, _ := slices.BinarySearch(needed, id)
		return at[i], found[i]
	}

	bounds := make([]boundary, 0, len(relations))
	for _, rel := range relations {
		b := boundary{pbfRelation: rel, box: BBox{MinLat: math.Inf(1), MinLon: math.Inf(1), MaxLat: math.Inf(-1), MaxLon: math.Inf(-1)}}
		for _, w := range rel.ways {
			refs := wayRefs[w]
			for i := 1; i < len(refs); i++ {
				a, okA := nodeAt(refs[i-1])
				c, okC := nodeAt(refs[i])
				if !okA || !okC {
					continue
				}
				b.ends = append(b.ends, a, c)
				for _, e := range [2]Point{a, c} {
					b.box.MinLat, b.box.MaxLat = math.Min(b.box.MinLat, e.Lat), math.Max(b.box.MaxLat, e.Lat)
					b.box.MinLon, b.box.MaxLon = math.Min(b.box.MinLon, e.Lon), math.Max(b.box.MaxLon, e.Lon)
				}
			}
		}
		if len(b.ends) > 0 {
			bounds = append(bounds, b)
		}
	}
	towns, streets = locate(points, roads, labels, places, bounds)
	return towns, streets, nil
}

// Only places inside the point's own boundary.
func locate(points []Point, roads *segments, labels []string, places []pbfPlace, bounds []boundary) (towns, streets []string) {
	towns, streets = make([]string, len(points)), make([]string, len(points))
	within := func(p Point) int {
		county := -1
		for i := range bounds {
			if !bounds[i].contains(p) {
				continue
			}
			if bounds[i].level == adminTown {
				return i
			}
			if county < 0 {
				county = i
			}
		}
		return county
	}
	placeIn := make([]int, len(places))
	for i := range placeIn {
		placeIn[i] = -2
	}
	for i, p := range points {
		if len(roads.segU) > 0 {
			if s := roads.Snap(p); s.Seg >= 0 {
				streets[i] = labels[s.Seg]
			}
		}
		in := within(p)
		if in < 0 {
			continue
		}
		towns[i] = bounds[in].name
		best := math.Inf(1)
		for j, pl := range places {
			if !bounds[in].box.contains(pl.lat, pl.lon) {
				continue
			}
			if placeIn[j] == -2 {
				placeIn[j] = within(Point{pl.lat, pl.lon})
			}
			if m := haversine(p.Lat, p.Lon, pl.lat, pl.lon); placeIn[j] == in && m < best {
				towns[i], best = pl.name, m
			}
		}
	}
	return towns, streets
}

func read(r io.ReadSeeker, keep pbfFilter, each func(pbfBlock)) error {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return readPBF(bufio.NewReaderSize(r, 1<<20), keep, each)
}
