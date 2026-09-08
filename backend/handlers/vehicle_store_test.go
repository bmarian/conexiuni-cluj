package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"path/filepath"
	"testing"
	"time"
)

func withTestDB(t *testing.T) {
	t.Helper()
	if err := database.Connect(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := database.InitSchemas(); err != nil {
		t.Fatalf("init schemas: %v", err)
	}
	t.Cleanup(func() { _ = database.DB.Close() })
}

func storedVehicle(id int, observedAt time.Time) models.Vehicle {
	v := models.Vehicle{
		ID: id, Label: "bus", Latitude: anchorLat, Longitude: anchorLon,
		Timestamp: observedAt.Format(time.RFC3339), VehicleType: 3,
		BikeAccessible: "UNKNOWN", WheelchairAccessible: "UNKNOWN",
		Speed: MinSpeedFloor, RawSpeed: 0, RouteID: 25, TripID: "25_1",
	}
	v.SetAnchor(anchorLat, anchorLon, observedAt.Format(time.RFC3339), observedAt.UnixMilli())
	return v
}

func TestStoreVehiclesRoundTripsMovementState(t *testing.T) {
	withTestDB(t)

	now := time.Now()
	v := storedVehicle(1, now)
	if err := storeVehiclesInDB([]models.Vehicle{v}); err != nil {
		t.Fatalf("store: %v", err)
	}

	got, err := getVehiclesFromDB(VehicleFilter{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d vehicles, want 1", len(got))
	}
	if got[0].RawSpeed != 0 || got[0].Speed != MinSpeedFloor {
		t.Fatalf("speeds = raw %v / smoothed %v, want 0 / %v", got[0].RawSpeed, got[0].Speed, MinSpeedFloor)
	}
	if got[0].StationarySince != v.StationarySince {
		t.Fatalf("StationarySince = %q, want %q", got[0].StationarySince, v.StationarySince)
	}
	lat, lon := got[0].Anchor()
	if lat != anchorLat || lon != anchorLon {
		t.Fatalf("anchor = (%v, %v), want (%v, %v)", lat, lon, anchorLat, anchorLon)
	}
}

// Before retention existed, every vehicle ever seen came back on the cache-hit
// path and got re-served as if it were live.
func TestGetVehiclesDropsRowsPastRetention(t *testing.T) {
	withTestDB(t)

	fresh := storedVehicle(1, time.Now())
	stale := storedVehicle(2, time.Now().Add(-vehicleRetention-time.Minute))
	if err := storeVehiclesInDB([]models.Vehicle{fresh, stale}); err != nil {
		t.Fatalf("store: %v", err)
	}

	got, err := getVehiclesFromDB(VehicleFilter{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("read %v, want only the fresh vehicle", got)
	}

	var remaining int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM vehicles").Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("%d rows left in the table, want the stale one pruned", remaining)
	}
}

// A vehicle Tranzy skips is re-served from its last known position, but with its
// original observation time, so repeated re-serving cannot keep it alive forever.
func TestGraceReservedVehicleKeepsAgeing(t *testing.T) {
	withTestDB(t)

	observedAt := time.Now().Add(-vehicleMissingGrace + 30*time.Second)
	if err := storeVehiclesInDB([]models.Vehicle{storedVehicle(1, observedAt)}); err != nil {
		t.Fatalf("store: %v", err)
	}

	got, err := getVehiclesFromDB(VehicleFilter{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d vehicles, want 1", len(got))
	}
	if err := storeVehiclesInDB(got); err != nil {
		t.Fatalf("re-store: %v", err)
	}

	after, err := getVehiclesFromDB(VehicleFilter{})
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if after[0].ObservedAt() != observedAt.UnixMilli() {
		t.Fatalf("ObservedAt = %d, want it unchanged at %d", after[0].ObservedAt(), observedAt.UnixMilli())
	}
}
