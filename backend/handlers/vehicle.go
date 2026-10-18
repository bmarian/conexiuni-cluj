package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/sources"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	// Wider than GPS jitter, narrower than a bus bay.
	StationaryRadiusMeters = 30.0

	// Older rows are neither served nor kept.
	vehicleRetention = 15 * time.Minute
)

const vehicleColumns = `id, label, latitude, longitude, timestamp, vehicle_type, bike_accessible, ` +
	`wheelchair_accessible, speed, route_id, trip_id, raw_speed, anchor_lat, anchor_lon, anchor_at, observed_at, ` +
	`prev_trip_id, trip_changed_at`

type VehicleFilter struct {
	RouteID *int
	TripID  *string
	TripIDs []string
}

func (city *City) observeVehicles(source sources.Realtime) ([]models.Vehicle, error) {
	vehicles, err := source.Vehicles()
	if err != nil {
		city.noteFeedError(err)
		return nil, err
	}
	smoothed, err := city.smoothVehicles(vehicles)
	if err != nil {
		return nil, err
	}
	city.log("vehicles: fetched %d", len(smoothed))
	city.noteFeed(len(vehicles), smoothed)
	go city.ObserveVehicleSegmentTravelTimes(city.Location, smoothed)
	if err := city.storeVehiclesInDB(smoothed); err != nil {
		city.log("vehicles: store: %v", err)
	}
	city.RecomputeLive(smoothed, time.Now())
	return smoothed, nil
}

func (city *City) smoothVehicles(apiVehicles []models.Vehicle) ([]models.Vehicle, error) {
	now := time.Now().In(city.Location)

	dbVehicles, err := city.getVehiclesFromDB(VehicleFilter{})
	if err != nil {
		for i := range apiVehicles {
			v := &apiVehicles[i]
			v.SetAnchor(v.Latitude, v.Longitude, v.Timestamp, parseVehicleTime(v.Timestamp, now))
		}
		return apiVehicles, nil
	}

	dbMap := make(map[int]models.Vehicle, len(dbVehicles))
	for _, dbV := range dbVehicles {
		dbMap[dbV.ID] = dbV
	}

	apiMap := make(map[int]bool, len(apiVehicles))
	for i := range apiVehicles {
		v := &apiVehicles[i]
		apiMap[v.ID] = true
		prev, exists := dbMap[v.ID]
		applyAnchor(v, prev, exists, now)
		carryTripChange(v, prev, exists)
	}

	// A vehicle the feed dropped from one poll keeps its last position for one more.
	missingGrace := 2 * livePollInterval(city)
	for _, dbV := range dbVehicles {
		if apiMap[dbV.ID] {
			continue
		}
		if now.Sub(time.UnixMilli(dbV.ObservedAt())) <= missingGrace {
			apiVehicles = append(apiVehicles, dbV)
		}
	}
	return apiVehicles, nil
}

// Dwell is measured from the anchor, not the last fix, so a bus inching forward still reads as waiting.
func applyAnchor(v *models.Vehicle, prev models.Vehicle, hadPrev bool, now time.Time) {
	observedAt := parseVehicleTime(v.Timestamp, now)
	if !hadPrev || prev.StationarySince == "" {
		v.SetAnchor(v.Latitude, v.Longitude, v.Timestamp, observedAt)
		return
	}
	anchorLat, anchorLon := prev.Anchor()
	if haversineMeters(anchorLat, anchorLon, v.Latitude, v.Longitude) >= StationaryRadiusMeters {
		v.SetAnchor(v.Latitude, v.Longitude, v.Timestamp, observedAt)
		return
	}
	v.SetAnchor(anchorLat, anchorLon, prev.StationarySince, observedAt)
}

// The feed flips the trip as the bus reaches the terminus, so the flip time is its arrival.
func carryTripChange(v *models.Vehicle, prev models.Vehicle, hadPrev bool) {
	if !hadPrev {
		return
	}
	if prev.TripID == v.TripID {
		v.PrevTripID = prev.PrevTripID
		v.TripChangedAt = prev.TripChangedAt
		return
	}
	v.PrevTripID = prev.TripID
	v.TripChangedAt = v.Timestamp
}

func parseVehicleTime(ts string, fallback time.Time) int64 {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.UnixMilli()
	}
	return fallback.UnixMilli()
}

func (city *City) getVehiclesFromDB(filter VehicleFilter) ([]models.Vehicle, error) {
	conditions := []string{"observed_at >= ?"}
	args := []any{vehicleRetentionCutoff()}
	if filter.RouteID != nil {
		conditions = append(conditions, "route_id = ?")
		args = append(args, *filter.RouteID)
	} else if filter.TripID != nil {
		conditions = append(conditions, "trip_id = ?")
		args = append(args, *filter.TripID)
	} else if len(filter.TripIDs) > 0 {
		ph := strings.Repeat("?,", len(filter.TripIDs))
		conditions = append(conditions, "trip_id IN ("+ph[:len(ph)-1]+")")
		for _, id := range filter.TripIDs {
			args = append(args, id)
		}
	}
	return queryRows(city.DB, `SELECT `+vehicleColumns+` FROM vehicles`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Vehicle, error) {
			var v models.Vehicle
			var anchorLat, anchorLon float64
			var anchorAt string
			var observedAt int64
			err := rows.Scan(&v.ID, &v.Label, &v.Latitude, &v.Longitude, &v.Timestamp, &v.VehicleType,
				&v.BikeAccessible, &v.WheelchairAccessible, &v.Speed, &v.RouteID, &v.TripID,
				&v.RawSpeed, &anchorLat, &anchorLon, &anchorAt, &observedAt, &v.PrevTripID, &v.TripChangedAt)
			if err != nil {
				return v, err
			}
			v.SetAnchor(anchorLat, anchorLon, anchorAt, observedAt)
			return v, nil
		})
}

func vehicleRetentionCutoff() int64 {
	return time.Now().Add(-vehicleRetention).UnixMilli()
}

func (city *City) storeVehiclesInDB(vehicles []models.Vehicle) error {
	err := batchExec(city.DB, `
		INSERT OR REPLACE INTO vehicles
		(`+vehicleColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, v := range vehicles {
				anchorLat, anchorLon := v.Anchor()
				if _, err := stmt.Exec(v.ID, v.Label, v.Latitude, v.Longitude, v.Timestamp, v.VehicleType,
					v.BikeAccessible, v.WheelchairAccessible, v.Speed, v.RouteID, v.TripID,
					v.RawSpeed, anchorLat, anchorLon, v.StationarySince, v.ObservedAt(), v.PrevTripID, v.TripChangedAt); err != nil {
					return fmt.Errorf("error inserting vehicle: %w", err)
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	_, err = city.DB.Exec(`DELETE FROM vehicles WHERE observed_at < ?`, vehicleRetentionCutoff())
	return err
}
