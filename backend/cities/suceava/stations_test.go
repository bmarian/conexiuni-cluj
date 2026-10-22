package suceava

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"math"
	"testing"
)

// A zip line north from Obcini, a kilometre between stops, and stops named like its own far away.
func testSnapshot() *feeds.Snapshot {
	stop := func(id int, name string, lat float64) models.Stop {
		return models.Stop{StopID: id, StopName: name, StopLat: lat, StopLon: 26.25}
	}
	snap := &feeds.Snapshot{Stops: []models.Stop{
		stop(1, "Obcini", 47.640), stop(2, "Spitalul Judetean", 47.649), stop(3, "Metro", 47.658), stop(4, "Metro", 47.900), stop(5, "Departe", 48.100),
	}}
	for i, id := range []int{1, 2, 3} {
		snap.TripStops = append(snap.TripStops, models.TripStop{TripID: "1_0", StopID: id, StopSequence: i})
	}
	return snap
}

func TestStationsTakeTheNearestStopOfTheirName(t *testing.T) {
	p := newPlacer(testSnapshot(), nil, nil)
	placed, counts := p.place([]pageLine{{short: "M1", directions: [][]string{{"OBCINI", "SPITAL", "FARA NUME", "METRO"}}}})
	dir := placed[0][0]
	if len(dir) != 4 || dir[1].stop.StopID != 2 || dir[3].stop.StopID != 3 {
		t.Fatalf("placed %+v", dir)
	}
	// Nothing names the station between Spital and Metro, so it sits halfway.
	if dir[2].from != spaced || math.Abs(dir[2].at.Lat-47.6535) > 1e-6 {
		t.Fatalf("unnamed station %+v", dir[2])
	}
	if counts[fromZip] != 3 || counts[spaced] != 1 {
		t.Fatalf("counts %v", counts)
	}
}

func TestALineWithoutItsEndIsLeftOut(t *testing.T) {
	p := newPlacer(testSnapshot(), nil, nil)
	placed, counts := p.place([]pageLine{{short: "M1", directions: [][]string{{"OBCINI", "SPITAL", "CAPAT"}}}})
	if placed[0] != nil || counts[unplaced] != 1 {
		t.Fatalf("placed %+v, counts %v", placed[0], counts)
	}
}

func TestAStopFarFromItsNeighboursIsAnotherOfThatName(t *testing.T) {
	p := newPlacer(testSnapshot(), nil, nil)
	placed, _ := p.place([]pageLine{{short: "M1", directions: [][]string{{"OBCINI", "DEPARTE", "METRO"}}}})
	if dir := placed[0][0]; len(dir) != 3 || dir[1].stop != nil || dir[1].from != spaced {
		t.Fatalf("placed %+v", dir)
	}
}
