package handlers

import (
	"conexiuni-cluj/models"
	"testing"
	"time"
)

const (
	anchorLat = 46.77415 // P-ța M. Viteazul Vest, terminus of route 32 dir 1
	anchorLon = 23.59032
	// ~11 m north at this latitude: inside StationaryRadiusMeters.
	creepStep = 0.0001
)

func vehicleAt(lat, lon float64, ts string) models.Vehicle {
	return models.Vehicle{ID: 1, Latitude: lat, Longitude: lon, Timestamp: ts}
}

func at(minute int) string {
	return time.Date(2026, 9, 8, 10, minute, 0, 0, time.UTC).Format(time.RFC3339)
}

func TestApplyAnchorFirstSightingReadsAsMoving(t *testing.T) {
	v := vehicleAt(anchorLat, anchorLon, at(0))
	applyAnchor(&v, models.Vehicle{}, false, time.Now())

	if v.StationarySince != at(0) {
		t.Fatalf("StationarySince = %q, want %q", v.StationarySince, at(0))
	}
	lat, lon := v.Anchor()
	if lat != anchorLat || lon != anchorLon {
		t.Fatalf("anchor = (%v, %v), want (%v, %v)", lat, lon, anchorLat, anchorLon)
	}
}

func TestApplyAnchorAccumulatesDwellWhileParked(t *testing.T) {
	prev := vehicleAt(anchorLat, anchorLon, at(0))
	applyAnchor(&prev, models.Vehicle{}, false, time.Now())

	// Eight minutes of GPS jitter around the same bay: a full terminus layover.
	for minute := 1; minute <= 8; minute++ {
		jitter := float64(minute%3) * 0.00005 // ~5 m
		v := vehicleAt(anchorLat+jitter, anchorLon-jitter, at(minute))
		applyAnchor(&v, prev, true, time.Now())
		prev = v
	}

	if prev.StationarySince != at(0) {
		t.Fatalf("StationarySince = %q, want it pinned to %q for the whole layover", prev.StationarySince, at(0))
	}
}

func TestApplyAnchorResetsOnceTheVehicleLeaves(t *testing.T) {
	prev := vehicleAt(anchorLat, anchorLon, at(0))
	applyAnchor(&prev, models.Vehicle{}, false, time.Now())

	v := vehicleAt(anchorLat+0.0005, anchorLon, at(1)) // ~55 m
	applyAnchor(&v, prev, true, time.Now())

	if v.StationarySince != at(1) {
		t.Fatalf("StationarySince = %q, want %q after moving clear of the anchor", v.StationarySince, at(1))
	}
	lat, _ := v.Anchor()
	if lat != anchorLat+0.0005 {
		t.Fatalf("anchor lat = %v, want it moved to %v", lat, anchorLat+0.0005)
	}
}

// The reason the anchor is carried across polls instead of comparing consecutive
// fixes: a bus creeping out of the terminus never moves far enough between two
// polls to count as moving, but it does leave the circle it started in.
func TestApplyAnchorCatchesSlowCreepOutOfTheTerminus(t *testing.T) {
	prev := vehicleAt(anchorLat, anchorLon, at(0))
	applyAnchor(&prev, models.Vehicle{}, false, time.Now())

	for step := 1; step <= 3; step++ {
		v := vehicleAt(anchorLat+float64(step)*creepStep, anchorLon, at(step))
		applyAnchor(&v, prev, true, time.Now())
		prev = v
	}

	if prev.StationarySince == at(0) {
		t.Fatal("StationarySince still pinned to the layover start; ~33 m of creep should re-anchor")
	}
}

func TestApplyAnchorFallsBackWhenTheTimestampIsUnparseable(t *testing.T) {
	now := time.Date(2026, 9, 8, 10, 30, 0, 0, time.UTC)
	v := vehicleAt(anchorLat, anchorLon, "not-a-timestamp")
	applyAnchor(&v, models.Vehicle{}, false, now)

	if v.ObservedAt() != now.UnixMilli() {
		t.Fatalf("ObservedAt = %d, want the server clock %d", v.ObservedAt(), now.UnixMilli())
	}
}
