package stpt

import (
	"fmt"
	"math"
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

func serve(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestReadsTheLiveMap(t *testing.T) {
	data, err := os.ReadFile("testdata/vehicles.json")
	if err != nil {
		t.Fatal(err)
	}
	m := ids{}
	vehicles, err := New(serve(t, data).URL, m).Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	if len(vehicles) != 77 {
		t.Fatalf("%d vehicles", len(vehicles))
	}
	v := vehicles[0]
	route, _ := m.ExternalID("route", "R_AB_E8")
	if v.Label != "GLM" || v.RouteID != route || v.TripID != fmt.Sprint(route, "_0") {
		t.Fatalf("first vehicle %+v", v)
	}
	if v.Speed != 17 || v.Timestamp != "2026-10-18T18:18:47Z" {
		t.Fatalf("first vehicle speed %f at %s", v.Speed, v.Timestamp)
	}
	if math.Abs(v.Latitude-45.7504) > 0.001 || math.Abs(v.Longitude-21.2783) > 0.001 {
		t.Fatalf("first vehicle at %f,%f", v.Latitude, v.Longitude)
	}
}

func TestSkipsVehiclesWithoutARouteOrDirection(t *testing.T) {
	body := []byte(`{"success":true,"data":{"vehicles":[
		{"id":"a","routeId":"R_TV_1","directionId":"1"},
		{"id":"b","routeId":"","directionId":"0"},
		{"id":"c","routeId":"R_TV_1","directionId":""},
		{"id":"","routeId":"R_TV_1","directionId":"0"}]}}`)
	vehicles, err := New(serve(t, body).URL, ids{}).Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	if len(vehicles) != 1 || vehicles[0].Label != "a" {
		t.Fatalf("kept %+v", vehicles)
	}
}

func TestAFailedAnswerIsAnError(t *testing.T) {
	if _, err := New(serve(t, []byte(`{"success":false}`)).URL, ids{}).Vehicles(); err == nil {
		t.Fatal("a failed answer was read as no buses")
	}
}
