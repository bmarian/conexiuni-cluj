package gtfsrt

import (
	"fmt"
	"math"
	"os"
	"testing"
)

// Stable ints per kind and external id, allocated as met.
type ids map[string]int

func (m ids) ExternalID(kind, external string) (int, error) {
	key := kind + ":" + external
	if _, ok := m[key]; !ok {
		m[key] = len(m) + 1
	}
	return m[key], nil
}

// Stops per "{route}_{direction}" pattern.
type patterns map[string][]int

func (p patterns) StopIndex(routeID, direction, stopID int) int {
	for i, id := range p[fmt.Sprintf("%d_%d", routeID, direction)] {
		if id == stopID {
			return i
		}
	}
	return -1
}

func TestDecodesTheFeed(t *testing.T) {
	data, err := os.ReadFile("testdata/vehicle_positions.pb")
	if err != nil {
		t.Fatal(err)
	}
	feed, err := decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if feed.timestamp != 1792330092 || len(feed.positions) != 472 {
		t.Fatalf("header %d, %d positions", feed.timestamp, len(feed.positions))
	}
	p := feed.positions[0]
	if p.routeID != "PV9_466" || p.stopID != "PV9_705" || p.vehicleID != "IF23POB" {
		t.Fatalf("first position %+v", p)
	}
	if p.direction != -1 || p.hasSpeed || p.timestamp != 1792330083 || p.stopSequence < 0 {
		t.Fatalf("first position %+v", p)
	}
	if math.Abs(p.lat-44.5376) > 0.001 || math.Abs(p.lon-25.9434) > 0.001 {
		t.Fatalf("first position at %f,%f", p.lat, p.lon)
	}
	for _, p := range feed.positions {
		if p.routeID == "" || p.vehicleID == "" || p.stopID == "" {
			t.Fatalf("incomplete position %+v", p)
		}
	}
}

func TestPlacesBusesByTheirNextStop(t *testing.T) {
	m := ids{}
	route, _ := m.ExternalID("route", "467")
	onlyOut, _ := m.ExternalID("stop", "out")
	terminus, _ := m.ExternalID("stop", "end")
	s := New("", m)
	s.Placement = patterns{
		fmt.Sprint(route, "_0"): {terminus, 10, onlyOut, 12},
		fmt.Sprint(route, "_1"): {12, 10, terminus},
	}
	feed := feedMessage{timestamp: 100, positions: []position{
		{routeID: "467", stopID: "out", vehicleID: "a", direction: -1, stopSequence: 2},
		{routeID: "467", stopID: "end", vehicleID: "b", direction: -1, stopSequence: 2},
		{routeID: "467", stopID: "end", vehicleID: "c", direction: -1, stopSequence: 0},
		{routeID: "467", stopID: "elsewhere", vehicleID: "d", direction: -1, stopSequence: 3},
		{routeID: "467", stopID: "end", vehicleID: "e", direction: -1, stopSequence: 5},
		{routeID: "467", stopID: "", vehicleID: "f", direction: 1},
	}}
	vehicles, err := s.vehicles(feed, 100)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, v := range vehicles {
		got[v.Label] = v.TripID
	}
	want := map[string]string{
		"a": fmt.Sprint(route, "_0"), // served by one direction
		"b": fmt.Sprint(route, "_1"), // a shared terminus, settled by the sequence number
		"c": fmt.Sprint(route, "_0"),
		"f": fmt.Sprint(route, "_1"), // the feed's own direction
	}
	if len(got) != len(want) {
		t.Fatalf("placed %v, want %v", got, want)
	}
	for label, trip := range want {
		if got[label] != trip {
			t.Errorf("%s on %q, want %q", label, got[label], trip)
		}
	}
}

func TestSpeedComesFromTheFeedOrFromTwoReadings(t *testing.T) {
	s := New("", ids{})
	first := feedMessage{positions: []position{
		{routeID: "1", vehicleID: "v", direction: 0, lat: 44.4000, lon: 26.1000, timestamp: 1000},
		{routeID: "1", vehicleID: "w", direction: 0, lat: 44.4000, lon: 26.1000, timestamp: 1000, speed: 10, hasSpeed: true},
	}}
	vehicles, _ := s.vehicles(first, 1000)
	if vehicles[0].Speed != 0 || vehicles[1].Speed != 36 {
		t.Fatalf("first reading speeds %f %f", vehicles[0].Speed, vehicles[1].Speed)
	}
	// 60 s later, about 500 m north: 30 km/h.
	second := feedMessage{positions: []position{
		{routeID: "1", vehicleID: "v", direction: 0, lat: 44.4045, lon: 26.1000, timestamp: 1060},
	}}
	vehicles, _ = s.vehicles(second, 1060)
	if math.Abs(vehicles[0].Speed-30) > 1 {
		t.Fatalf("derived speed %f", vehicles[0].Speed)
	}
	if vehicles[0].Timestamp != "1970-01-01T00:17:40Z" {
		t.Fatalf("timestamp %s", vehicles[0].Timestamp)
	}
}
