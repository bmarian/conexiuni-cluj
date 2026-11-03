package modeshift

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type ids map[string]int

func (m ids) ExternalID(kind, external string) (int, error) {
	key := kind + ":" + external
	if _, ok := m[key]; !ok {
		m[key] = len(m) + 1
	}
	return m[key], nil
}

type runs map[string]int

func (m runs) RunDirections(runIDs []string) (map[string]int, error) {
	out := map[string]int{}
	for _, id := range runIDs {
		if direction, ok := m[id]; ok {
			out[id] = direction
		}
	}
	return out, nil
}

func TestPlacesBusesOnTheirTripsDirection(t *testing.T) {
	data, err := os.ReadFile("testdata/vehicles.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(data) }))
	t.Cleanup(server.Close)
	m := ids{}
	vehicles, err := New(server.URL, m, runs{"8.714": 1, "8.745": 0}).Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	if len(vehicles) != 2 {
		t.Fatalf("%d vehicles, want the 2 whose trip the import listed", len(vehicles))
	}
	route, _ := m.ExternalID("route", "21")
	standing, moving := vehicles[0], vehicles[1]
	if standing.Label != "R84" || standing.RouteID != route || standing.TripID != "1_1" || standing.Speed != 0 || standing.Timestamp != "2026-10-24T11:21:14Z" {
		t.Errorf("standing bus %+v", standing)
	}
	if moving.TripID != "1_0" || moving.Speed < 35.9 || moving.Speed > 36 {
		t.Errorf("moving bus %+v, want %.1f km/h", moving, 9.98*3.6)
	}
}
