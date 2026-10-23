package web

import "testing"

func TestLogURLHidesPlaces(t *testing.T) {
	cases := map[string]string{
		"/api/cluj/plan?from_lat=46.77&from_lng=23.6&to_lat=46.75&to_lng=23.58&time=2026-10-23T08:00": "/api/cluj/plan?from_lat=-&from_lng=-&to_lat=-&to_lng=-&time=-",
		"/api/cluj/places?q=Strada%20Mea%2012&limit=10":                                               "/api/cluj/places?q=-&limit=-",
		"/api/iasi/places?lat=47.16&lon=27.58":                                                        "/api/iasi/places?lat=-&lon=-",
		"/cluj/plan?lat=46.77&lon=23.6&name=Acasa&originName=Birou":                                   "/cluj/plan?lat=-&lon=-&name=-&originName=-",
		"/cluj/PLAN/?q=Acasa":                            "/cluj/PLAN/?q=-",
		"/cluj/plan":                                     "/cluj/plan",
		"/api/cluj/live?stop_ids=1,2&once=1":             "/api/cluj/live?stop_ids=1,2&once=1",
		"/api/cluj/shapes?format=polyline&shape_ids=1_0": "/api/cluj/shapes?format=polyline&shape_ids=1_0",
		"/api/cluj/planner-notes?x=1":                    "/api/cluj/planner-notes?x=1",
	}
	for in, want := range cases {
		if got := LogURL(in); got != want {
			t.Errorf("LogURL(%s) = %s, want %s", in, got, want)
		}
	}
}
