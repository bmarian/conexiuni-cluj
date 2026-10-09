package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"conexiuni-cluj/models"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func streamBus(id int, lat float64, label string) models.Vehicle {
	return models.Vehicle{ID: id, Label: label, Latitude: lat, Longitude: 23.5, Timestamp: "t", VehicleType: 3,
		BikeAccessible: "UNKNOWN", WheelchairAccessible: "UNKNOWN", Speed: 20, RawSpeed: 20, RouteID: 1, TripID: "1_0"}
}

func TestVehicleDeltasSendASnapshotThenOnlyWhatMoved(t *testing.T) {
	d := &vehicleDeltas{}

	event, payload := d.next([]models.Vehicle{streamBus(1, 46.1, "a"), streamBus(2, 46.2, "b")})
	snapshot, ok := payload.(streamSnapshot)
	if event != "snapshot" || !ok || len(snapshot.Vehicles) != 2 || snapshot.Vehicles[0].Label == nil {
		t.Fatalf("first event = %q %+v, want a snapshot of both buses with their labels", event, payload)
	}

	if event, _ := d.next([]models.Vehicle{streamBus(1, 46.100001, "a"), streamBus(2, 46.2, "b")}); event != "" {
		t.Fatalf("unchanged buses sent %q", event)
	}

	event, payload = d.next([]models.Vehicle{streamBus(1, 46.15, "a"), streamBus(3, 46.3, "c")})
	delta := payload.(streamDelta)
	if event != "delta" || len(delta.Upsert) != 2 || len(delta.Remove) != 1 || delta.Remove[0] != 2 {
		t.Fatalf("delta = %q %+v", event, delta)
	}
	if delta.Upsert[0].ID != 1 || delta.Upsert[0].Label != nil || delta.Upsert[0].Latitude != 46.15 {
		t.Fatalf("moved bus = %+v, want its new position without the label it already has", delta.Upsert[0])
	}
	if delta.Upsert[1].ID != 3 || delta.Upsert[1].Label == nil || *delta.Upsert[1].Label != "c" {
		t.Fatalf("new bus = %+v, want it whole", delta.Upsert[1])
	}

	_, payload = d.next([]models.Vehicle{streamBus(1, 46.15, "a2"), streamBus(3, 46.3, "c")})
	if up := payload.(streamDelta).Upsert; len(up) != 1 || up[0].Label == nil || *up[0].Label != "a2" {
		t.Fatalf("relabelled = %+v", up)
	}
}

func TestStreamTripIDsExpandsRoutes(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(strings.Join(streamTripIDs(c), ",")) })
	resp, err := app.Test(httptest.NewRequest("GET", "/?trip_ids=5_0&route_ids=7,x", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "5_0,7_0,7_1" {
		t.Fatalf("trips = %s", body)
	}
}

func TestGzippedEventsArriveWhole(t *testing.T) {
	var wire bytes.Buffer
	w := bufio.NewWriter(&wire)
	gz, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
	out := &eventWriter{w: w, gz: gz}

	data, _ := json.Marshal(streamSnapshot{Vehicles: []streamVehicle{toStreamVehicle(streamBus(1, 46.1, "a"), true)}})
	event := "event: snapshot\ndata: " + string(data) + "\n\n"
	if err := out.write(event); err != nil {
		t.Fatal(err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(wire.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(event))
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatalf("event not readable before the stream closed: %v", err)
	}
	if string(got) != event {
		t.Fatalf("read %q", got)
	}
}
