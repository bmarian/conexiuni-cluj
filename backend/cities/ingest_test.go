package cities

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"testing"
	"time"
)

type swapStatic struct{ snap *feeds.Snapshot }

func (s *swapStatic) Snapshot() (*feeds.Snapshot, error) {
	copied := *s.snap
	return &copied, nil
}
func (s *swapStatic) Cadence() time.Duration { return time.Hour }

func oneLineSnapshot(endLon float64) *feeds.Snapshot {
	snap := &feeds.Snapshot{
		Routes: []models.Route{{RouteID: 1, RouteShortName: "1", RouteLongName: "A - B", RouteType: 3}},
		Stops: []models.Stop{
			{StopID: 1, StopName: "A", StopLat: 46, StopLon: 24},
			{StopID: 2, StopName: "B", StopLat: 46, StopLon: endLon},
		},
		Trips: []models.Trip{{TripID: "1_0", RouteID: 1, ShapeID: "1_0"}},
		Shapes: []models.Shape{
			{ShapeID: "1_0", ShapePtLat: 46, ShapePtLon: 24, ShapePtSequence: 1},
			{ShapeID: "1_0", ShapePtLat: 46, ShapePtLon: endLon, ShapePtSequence: 2},
		},
		TripStops: []models.TripStop{{TripID: "1_0", StopID: 1, StopSequence: 1}, {TripID: "1_0", StopID: 2, StopSequence: 2}},
	}
	return snap
}

func TestANewNetworkReplacesTheTrackedGeometry(t *testing.T) {
	static := &swapStatic{snap: oneLineSnapshot(24.01)}
	eng := newNamedTestCity(t, "t", static)
	if err := eng.Ingest.Run(time.Now()); err != nil {
		t.Fatal(err)
	}
	if shape, ok := eng.Live.Ride("1_0", 1, 2); !ok || shape[1].ShapePtLon != 24.01 {
		t.Fatalf("first shape: %+v %v", shape, ok)
	}
	static.snap = oneLineSnapshot(24.02)
	if err := eng.ForgetFetches(); err != nil {
		t.Fatal(err)
	}
	if err := eng.Ingest.Run(time.Now()); err != nil {
		t.Fatal(err)
	}
	if shape, ok := eng.Live.Ride("1_0", 1, 2); !ok || shape[1].ShapePtLon != 24.02 {
		t.Fatalf("the shape after the new network still ends at %v", shape[1].ShapePtLon)
	}
}
