package cities

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"slices"
	"testing"
	"time"
)

func TestATimetableNamingTheFirstVisibleStopCountsFromIt(t *testing.T) {
	eng := newNamedTestCity(t, "t", &swapStatic{snap: threeStopSnapshot()})
	if err := eng.Ingest.Run(time.Now()); err != nil {
		t.Fatal(err)
	}
	day := `{"entries":[{"departure_in":"08:00","departure_out":"08:30"}]}`
	if _, err := eng.DB.Exec(`INSERT INTO timetable VALUES ('1', 'A - C', 'B', 'C', ?, ?, ?)`, day, day, day); err != nil {
		t.Fatal(err)
	}
	if err := eng.HideStops([]int{1}, true); err != nil {
		t.Fatal(err)
	}
	if got := eng.Availability().TimetableStart("1_0"); got != 1 {
		t.Fatalf("timetable starts at %d, want 1", got)
	}
	bundle, _, err := eng.Network.Current()
	if err != nil {
		t.Fatal(err)
	}
	if first := bundle.Routes[0].Offsets["weekday"][8][0]; first != 0 {
		t.Fatalf("first visible stop is %d s after the timetable", first)
	}
}

func threeStopSnapshot() *feeds.Snapshot {
	snap := oneLineSnapshot(24.02)
	snap.Stops = append(snap.Stops, models.Stop{StopID: 3, StopName: "C", StopLat: 46, StopLon: 24.04})
	snap.Shapes = append(snap.Shapes, models.Shape{ShapeID: "1_0", ShapePtLat: 46, ShapePtLon: 24.04, ShapePtSequence: 3})
	snap.TripStops = append(snap.TripStops, models.TripStop{TripID: "1_0", StopID: 3, StopSequence: 3})
	return snap
}

func TestHidingAStopTakesItOutOfTheBundle(t *testing.T) {
	eng := newNamedTestCity(t, "t", &swapStatic{snap: threeStopSnapshot()})
	if err := eng.Ingest.Run(time.Now()); err != nil {
		t.Fatal(err)
	}
	trip := func() (models.NetworkTrip, models.NetworkRoute, []models.Stop) {
		t.Helper()
		bundle, _, err := eng.Network.Current()
		if err != nil {
			t.Fatal(err)
		}
		return bundle.Routes[0].Trips[0], bundle.Routes[0], bundle.Stops
	}
	shown, route, _ := trip()
	day := route.Offsets["weekday"][8]
	fromTheStart := day[1] + day[2]

	if err := eng.HideStops([]int{1}, true); err != nil {
		t.Fatal(err)
	}
	hidden, route, stops := trip()
	if !slices.Equal(hidden.StopIDs, []int{2, 3}) {
		t.Fatalf("stops on the trip: %v", hidden.StopIDs)
	}
	if slices.ContainsFunc(stops, func(s models.Stop) bool { return s.StopID == 1 }) || eng.Availability().StopHasBuses(1) {
		t.Fatalf("stop 1 is still offered: %+v", stops)
	}
	offsets := route.Offsets["weekday"][8]
	if len(offsets) != 2 || offsets[0]+offsets[1] != day[0]+fromTheStart {
		t.Fatalf("offsets %v, all stops %v", offsets, day)
	}
	if instead, ok := eng.StopInstead(1); !ok || instead.StopID != 2 {
		t.Fatalf("instead of stop 1: %+v %v", instead, ok)
	}

	all, err := eng.Shapes([]string{"1_0"})
	if err != nil {
		t.Fatal(err)
	}
	if line := eng.Live.PassengerShapes(all); len(line) != 2 || line[0].ShapePtLon != 24.02 {
		t.Fatalf("drawn line: %+v", line)
	}
	if hidden.ShapeHash == shown.ShapeHash {
		t.Fatal("clients keep the old line: same shape hash")
	}

	if err := eng.HideStops([]int{1}, false); err != nil {
		t.Fatal(err)
	}
	if back, _, _ := trip(); !slices.Equal(back.StopIDs, shown.StopIDs) {
		t.Fatalf("stops after bringing it back: %v", back.StopIDs)
	}
}
