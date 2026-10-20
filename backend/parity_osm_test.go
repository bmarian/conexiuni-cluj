package main

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

type osmStop struct {
	id       int
	name     string
	lat, lon float64
}

// A street grid with named streets and cafés over every city's stops, written as an OSM PBF the router can read.
func writeSyntheticOSM(t *testing.T, path string, cities map[string][]osmStop) {
	t.Helper()
	w := &pbfWriter{}
	var ways []osmWay
	for _, name := range slices.Sorted(maps.Keys(cities)) {
		if stops := cities[name]; len(stops) > 0 {
			ways = append(ways, w.city(name, stops)...)
		}
	}
	for i := 0; i < len(w.nodes); i += 8000 {
		w.denseBlock(w.nodes[i:min(i+8000, len(w.nodes))])
	}
	for i := 0; i < len(ways); i += 500 {
		w.wayBlock(ways[i:min(i+500, len(ways))])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, w.out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func storedOSMStops(t *testing.T, dbDir string) map[string][]osmStop {
	t.Helper()
	dbs, _ := filepath.Glob(filepath.Join(dbDir, "*.db"))
	out := make(map[string][]osmStop, len(dbs))
	for _, db := range dbs {
		out[strings.TrimSuffix(filepath.Base(db), ".db")] = readOSMStops(t, db)
	}
	return out
}

func readOSMStops(t *testing.T, path string) []osmStop {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT stop_id, stop_name, stop_lat, stop_lon FROM stops ORDER BY stop_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []osmStop
	for rows.Next() {
		var s osmStop
		if err := rows.Scan(&s.id, &s.name, &s.lat, &s.lon); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

type osmNode struct {
	id       int64
	lat, lon float64
	tags     [][2]string
}

type osmWay struct {
	id   int64
	refs []int64
	tags [][2]string
}

type pbfWriter struct {
	out    []byte
	nodes  []osmNode
	nextID int64
}

func (w *pbfWriter) node(lat, lon float64, tags ...[2]string) int64 {
	w.nextID++
	w.nodes = append(w.nodes, osmNode{id: w.nextID, lat: lat, lon: lon, tags: tags})
	return w.nextID
}

func (w *pbfWriter) city(name string, stops []osmStop) []osmWay {
	south, west, north, east := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	lats := make([]float64, len(stops))
	lons := make([]float64, len(stops))
	for i, s := range stops {
		south, north = min(south, s.lat), max(north, s.lat)
		west, east = min(west, s.lon), max(east, s.lon)
		lats[i], lons[i] = s.lat, s.lon
	}
	const latStep, lonStep, margin = 0.0025, 0.0035, 0.015
	rows := int((north-south+2*margin)/latStep) + 1
	cols := int((east-west+2*margin)/lonStep) + 1
	grid := make([][]int64, rows)
	for i := range grid {
		grid[i] = make([]int64, cols)
		for j := range grid[i] {
			grid[i][j] = w.node(south-margin+float64(i)*latStep, west-margin+float64(j)*lonStep)
		}
	}
	var ways []osmWay
	for i := range rows {
		ways = append(ways, osmWay{id: w.nextID + int64(len(ways)) + 1, refs: grid[i],
			tags: [][2]string{{"highway", "residential"}, {"name", fmt.Sprintf("Strada Paralelă %d", i)}}})
	}
	for j := range cols {
		refs := make([]int64, rows)
		for i := range rows {
			refs[i] = grid[i][j]
		}
		ways = append(ways, osmWay{id: w.nextID + int64(len(ways)) + 1, refs: refs,
			tags: [][2]string{{"highway", "tertiary"}, {"name", fmt.Sprintf("Bulevardul Meridian %d", j)}}})
	}
	for i := range ways {
		ways[i].id += 10_000_000
	}
	sort.Float64s(lats)
	sort.Float64s(lons)
	w.node(lats[len(lats)/2], lons[len(lons)/2], [2]string{"place", "city"}, [2]string{"name", name})
	for i, s := range stops {
		if i%5 == 0 {
			w.node(s.lat+0.0004, s.lon+0.0004, [2]string{"amenity", "cafe"}, [2]string{"name", "Cafenea " + s.name})
		}
		if i%40 == 0 {
			w.node(s.lat-0.0006, s.lon, [2]string{"place", "suburb"}, [2]string{"name", fmt.Sprintf("Cartier %d", i/40)},
				[2]string{"addr:street", fmt.Sprintf("Strada Paralelă %d", i%rows)}, [2]string{"addr:housenumber", fmt.Sprint(i)})
		}
	}
	return ways
}

func (w *pbfWriter) denseBlock(nodes []osmNode) {
	strs := newStringTable()
	var ids, lats, lons []int64
	var kv []uint64
	var prevID, prevLat, prevLon int64
	for _, n := range nodes {
		lat, lon := int64(math.Round(n.lat*1e7)), int64(math.Round(n.lon*1e7))
		ids, lats, lons = append(ids, n.id-prevID), append(lats, lat-prevLat), append(lons, lon-prevLon)
		prevID, prevLat, prevLon = n.id, lat, lon
		for _, tag := range n.tags {
			kv = append(kv, strs.index(tag[0]), strs.index(tag[1]))
		}
		kv = append(kv, 0)
	}
	var dense []byte
	dense = pbPackedSints(dense, 1, ids)
	dense = pbPackedSints(dense, 8, lats)
	dense = pbPackedSints(dense, 9, lons)
	dense = pbPackedUints(dense, 10, kv)
	w.block(strs, pbBytes(nil, 2, dense))
}

func (w *pbfWriter) wayBlock(ways []osmWay) {
	strs := newStringTable()
	var group []byte
	for _, way := range ways {
		var keys, vals []uint64
		for _, tag := range way.tags {
			keys, vals = append(keys, strs.index(tag[0])), append(vals, strs.index(tag[1]))
		}
		refs := make([]int64, len(way.refs))
		var prev int64
		for i, r := range way.refs {
			refs[i], prev = r-prev, r
		}
		var b []byte
		b = pbUint(b, 1, uint64(way.id))
		b = pbPackedUints(b, 2, keys)
		b = pbPackedUints(b, 3, vals)
		b = pbPackedSints(b, 8, refs)
		group = pbBytes(group, 3, b)
	}
	w.block(strs, group)
}

func (w *pbfWriter) block(strs *stringTable, group []byte) {
	var table []byte
	for _, s := range strs.list {
		table = pbBytes(table, 1, []byte(s))
	}
	data := pbBytes(nil, 1, table)
	data = pbBytes(data, 2, group)
	blob := pbBytes(nil, 1, data)
	header := pbBytes(nil, 1, []byte("OSMData"))
	header = pbUint(header, 3, uint64(len(blob)))
	w.out = binary.BigEndian.AppendUint32(w.out, uint32(len(header)))
	w.out = append(append(w.out, header...), blob...)
}

type stringTable struct {
	list []string
	at   map[string]uint64
}

func newStringTable() *stringTable {
	return &stringTable{list: []string{""}, at: map[string]uint64{"": 0}}
}

func (s *stringTable) index(v string) uint64 {
	if i, ok := s.at[v]; ok {
		return i
	}
	s.at[v] = uint64(len(s.list))
	s.list = append(s.list, v)
	return s.at[v]
}

func pbUint(b []byte, field int, v uint64) []byte {
	return binary.AppendUvarint(binary.AppendUvarint(b, uint64(field)<<3), v)
}

func pbBytes(b []byte, field int, data []byte) []byte {
	b = binary.AppendUvarint(b, uint64(field)<<3|2)
	return append(binary.AppendUvarint(b, uint64(len(data))), data...)
}

func pbPackedUints(b []byte, field int, vs []uint64) []byte {
	var data []byte
	for _, v := range vs {
		data = binary.AppendUvarint(data, v)
	}
	return pbBytes(b, field, data)
}

func pbPackedSints(b []byte, field int, vs []int64) []byte {
	var data []byte
	for _, v := range vs {
		data = binary.AppendUvarint(data, uint64(v<<1)^uint64(v>>63))
	}
	return pbBytes(b, field, data)
}
