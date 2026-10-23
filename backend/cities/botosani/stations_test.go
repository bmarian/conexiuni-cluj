package botosani

import (
	"conexiuni-cluj/router"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Lower-case stations have a marker on each side.
func eastward(names ...string) []marker {
	var out []marker
	for i, name := range names {
		lon := 26.6 + float64(i)*0.004
		if name[0] >= 'a' {
			out = append(out, marker{id: name + "-south", name: name, at: router.Point{Lat: 47.7499, Lon: lon}}, marker{id: name + "-north", name: name, at: router.Point{Lat: 47.7501, Lon: lon}})
			continue
		}
		out = append(out, marker{id: name, name: name, at: router.Point{Lat: 47.75, Lon: lon}})
	}
	return out
}

func ids(runs [][]marker) [][]string {
	out := make([][]string, len(runs))
	for i, r := range runs {
		for _, m := range r {
			out[i] = append(out[i], m.id)
		}
	}
	return out
}

func TestEachWayStopsOnItsRightAndAStationNamedTwiceIsTheOneLeftOut(t *testing.T) {
	line := pageLine{markers: eastward("U", "s", "p", "R"), runs: []run{
		{label: outward, names: []string{"U", "s", "p", "R"}},
		// R again where s belongs, as on the page.
		{label: inward, names: []string{"R", "p", "R", "U"}},
	}}
	placed := placeRuns(line, streets{})
	want := [][]string{{"U", "s-south", "p-south", "R"}, {"R", "p-north", "s-north", "U"}}
	if got := ids(placed.runs); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("placed %v, want %v", got, want)
	}
	if len(placed.unused) > 0 || len(placed.missing) > 0 {
		t.Fatalf("unused %v, missing %v", placed.unused, placed.missing)
	}
}

func TestALoopPassesAStationOnceEachWay(t *testing.T) {
	line := pageLine{markers: eastward("A", "b", "C"), runs: []run{{label: "CURSA SCURTĂ", names: []string{"A", "b", "C", "b", "A"}}}}
	placed := placeRuns(line, streets{})
	if got, want := ids(placed.runs)[0], []string{"A", "b-south", "C", "b-north", "A"}; !slices.Equal(got, want) {
		t.Fatalf("placed %v, want %v", got, want)
	}
}

func TestRunsKeepRightOnTheRealRoads(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the OSM extract")
	}
	f, err := os.Open(filepath.Join("..", "..", "services", "osm", "romania-latest.osm.pbf"))
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	g, _, err := router.LoadRoads(f, router.BBox{MinLat: 47.73, MinLon: 26.60, MaxLat: 47.77, MaxLon: 26.68})
	if err != nil {
		t.Fatal(err)
	}
	lines, _ := fixturePage(t)
	want := [][][]string{
		{
			{"9", "11", "13", "15", "17", "19", "21", "23", "26", "28", "30", "32", "34"},
			{"34", "33", "31", "29", "27", "24", "22", "20", "18", "16", "14", "12", "10"},
		},
		{
			{"34", "33", "31", "29", "27", "24", "35", "38", "39", "42", "43"},
			{"44", "41", "40", "37", "36", "23", "26", "28", "30", "32", "34"},
			{"44", "41", "40", "37", "36", "23", "25", "35", "38", "39", "42", "43"},
		},
	}
	for i, line := range lines {
		placed := placeRuns(line, streets{g})
		if got := ids(placed.runs); !slices.EqualFunc(got, want[i], slices.Equal) {
			t.Errorf("%s: placed %v, want %v", line.short, got, want[i])
		}
		if len(placed.unused) > 0 || len(placed.missing) > 0 {
			t.Errorf("%s: unused %v, missing %v", line.short, placed.unused, placed.missing)
		}
	}
}
