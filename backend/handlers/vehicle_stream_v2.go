package handlers

import (
	"bufio"
	"compress/gzip"
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type streamVehicle struct {
	ID                   int     `json:"id"`
	TripID               string  `json:"trip_id"`
	RouteID              int     `json:"route_id"`
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	Timestamp            string  `json:"timestamp"`
	Speed                float64 `json:"speed"`
	RawSpeed             float64 `json:"raw_speed"`
	StationarySince      string  `json:"stationary_since"`
	Label                *string `json:"label,omitempty"`
	VehicleType          *int    `json:"vehicle_type,omitempty"`
	BikeAccessible       *string `json:"bike_accessible,omitempty"`
	WheelchairAccessible *string `json:"wheelchair_accessible,omitempty"`
}

type streamSnapshot struct {
	Vehicles []streamVehicle `json:"vehicles"`
}

type streamDelta struct {
	Upsert []streamVehicle `json:"upsert"`
	Remove []int           `json:"remove"`
}

func roundTo(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}

func roundedVehicle(v models.Vehicle) models.Vehicle {
	v.Latitude = roundTo(v.Latitude, 5)
	v.Longitude = roundTo(v.Longitude, 5)
	v.Speed = roundTo(v.Speed, 1)
	v.RawSpeed = roundTo(v.RawSpeed, 1)
	return v
}

func sameMotion(a, b models.Vehicle) bool {
	return a.TripID == b.TripID && a.RouteID == b.RouteID && a.Latitude == b.Latitude && a.Longitude == b.Longitude &&
		a.Timestamp == b.Timestamp && a.Speed == b.Speed && a.RawSpeed == b.RawSpeed && a.StationarySince == b.StationarySince
}

func sameLook(a, b models.Vehicle) bool {
	return a.Label == b.Label && a.VehicleType == b.VehicleType &&
		a.BikeAccessible == b.BikeAccessible && a.WheelchairAccessible == b.WheelchairAccessible
}

func toStreamVehicle(v models.Vehicle, withLook bool) streamVehicle {
	out := streamVehicle{
		ID: v.ID, TripID: v.TripID, RouteID: v.RouteID, Latitude: v.Latitude, Longitude: v.Longitude,
		Timestamp: v.Timestamp, Speed: v.Speed, RawSpeed: v.RawSpeed, StationarySince: v.StationarySince,
	}
	if withLook {
		label, vehicleType, bike, wheelchair := v.Label, v.VehicleType, v.BikeAccessible, v.WheelchairAccessible
		out.Label, out.VehicleType, out.BikeAccessible, out.WheelchairAccessible = &label, &vehicleType, &bike, &wheelchair
	}
	return out
}

type vehicleDeltas struct {
	sent    map[int]models.Vehicle
	started bool
}

func (d *vehicleDeltas) next(batch []models.Vehicle) (event string, payload any) {
	current := make(map[int]models.Vehicle, len(batch))
	for _, v := range batch {
		current[v.ID] = roundedVehicle(v)
	}
	if !d.started {
		d.started = true
		d.sent = current
		snapshot := streamSnapshot{Vehicles: make([]streamVehicle, 0, len(batch))}
		for _, v := range sortedVehicles(current) {
			snapshot.Vehicles = append(snapshot.Vehicles, toStreamVehicle(v, true))
		}
		return "snapshot", snapshot
	}
	delta := streamDelta{Upsert: []streamVehicle{}, Remove: []int{}}
	for _, v := range sortedVehicles(current) {
		before, known := d.sent[v.ID]
		switch {
		case !known:
			delta.Upsert = append(delta.Upsert, toStreamVehicle(v, true))
		case !sameLook(before, v):
			delta.Upsert = append(delta.Upsert, toStreamVehicle(v, true))
		case !sameMotion(before, v):
			delta.Upsert = append(delta.Upsert, toStreamVehicle(v, false))
		}
	}
	for id := range d.sent {
		if _, still := current[id]; !still {
			delta.Remove = append(delta.Remove, id)
		}
	}
	d.sent = current
	if len(delta.Upsert) == 0 && len(delta.Remove) == 0 {
		return "", nil
	}
	slices.Sort(delta.Remove)
	return "delta", delta
}

func sortedVehicles(m map[int]models.Vehicle) []models.Vehicle {
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	out := make([]models.Vehicle, len(ids))
	for i, id := range ids {
		out[i] = m[id]
	}
	return out
}

func streamTripIDs(c fiber.Ctx) []string {
	var ids []string
	for _, id := range strings.Split(c.Query("trip_ids"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	for _, id := range strings.Split(c.Query("route_ids"), ",") {
		if _, err := strconv.Atoi(strings.TrimSpace(id)); err == nil {
			id = strings.TrimSpace(id)
			ids = append(ids, id+OUTGOING_SUFFIX, id+INCOMING_SUFFIX)
		}
	}
	return ids
}

type eventWriter struct {
	w  *bufio.Writer
	gz *gzip.Writer
}

func (e *eventWriter) write(text string) error {
	if e.gz != nil {
		if _, err := e.gz.Write([]byte(text)); err != nil {
			return err
		}
		if err := e.gz.Flush(); err != nil {
			return err
		}
	} else if _, err := e.w.WriteString(text); err != nil {
		return err
	}
	return e.w.Flush()
}

func (e *eventWriter) close() {
	if e.gz != nil {
		_ = e.gz.Close()
		_ = e.w.Flush()
	}
}

func serveVehicleStreamV2(c fiber.Ctx, tripIDs []string) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Set("Vary", "Accept-Encoding")
	gzipped := strings.Contains(c.Get("Accept-Encoding"), "gzip")
	if gzipped {
		c.Set("Content-Encoding", "gzip")
	}
	sub, id := VehicleHub.Subscribe(tripIDs)
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer VehicleHub.Unsubscribe(id)
		out := &eventWriter{w: w}
		if gzipped {
			out.gz, _ = gzip.NewWriterLevel(w, gzip.BestSpeed)
		}
		defer out.close()
		deltas := &vehicleDeltas{}
		keep := time.NewTicker(25 * time.Second)
		defer keep.Stop()
		for {
			select {
			case batch, ok := <-sub.Ch():
				if !ok {
					return
				}
				event, payload := deltas.next(batch)
				if event == "" {
					continue
				}
				data, err := json.Marshal(payload)
				if err != nil {
					continue
				}
				if err := out.write(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)); err != nil {
					return
				}
			case <-keep.C:
				if err := out.write(": ping\n\n"); err != nil {
					return
				}
			}
		}
	})
}
