package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	VehicleCacheId = "VEHICLES"
	EmaAlpha       = 0.3
	MinSpeedFloor  = 7.0 // MIN_SPEED_KMH
)

type VehicleFilter struct {
	RouteID *int
	TripID  *string
	TripIDs []string
}

func GetVehicles(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter VehicleFilter) ([]models.Vehicle, error) {
	if filter.TripID != nil {
		normalized := NormalizeTripID(*filter.TripID)
		filter.TripID = &normalized
	}
	for i, id := range filter.TripIDs {
		filter.TripIDs[i] = NormalizeTripID(id)
	}

	opts := CacheOpts[[]models.Vehicle]{}
	if filter.RouteID != nil || len(filter.TripIDs) > 0 {
		f := filter
		tripSet := tripIDSet(f.TripIDs)
		opts.PostProcess = func(vs []models.Vehicle) []models.Vehicle {
			out := make([]models.Vehicle, 0)
			for _, v := range vs {
				if f.RouteID != nil && v.RouteID != *f.RouteID {
					continue
				}
				if tripSet != nil {
					if _, ok := tripSet[v.TripID]; !ok {
						continue
					}
				}
				out = append(out, v)
			}
			return out
		}
	}
	return HandleCached(VehicleCacheId, cacheShelfLife,
		func() ([]models.Vehicle, error) { return getVehiclesFromDB(filter) },
		func() ([]models.Vehicle, error) { return requestVehicles(tranzyClient, filter) },
		storeVehiclesInDB,
		opts,
	)
}

func requestVehicles(tranzyClient *tranzy.Client, filter VehicleFilter) ([]models.Vehicle, error) {
	vehicles, err := tranzyFetch[[]models.Vehicle](tranzyClient, "/vehicles")
	if err != nil {
		return nil, err
	}
	if vehicles == nil {
		vehicles = make([]models.Vehicle, 0)
	}

	tripSet := tripIDSet(filter.TripIDs)
	filtered := make([]models.Vehicle, 0)
	for _, v := range vehicles {
		if v.RouteID == -1 || v.TripID == "-1" {
			continue
		}
		v.TripID = NormalizeTripID(v.TripID)
		if filter.RouteID != nil && v.RouteID != *filter.RouteID {
			continue
		}
		if filter.TripID != nil && v.TripID != *filter.TripID {
			continue
		}
		if tripSet != nil {
			if _, ok := tripSet[v.TripID]; !ok {
				continue
			}
		}
		filtered = append(filtered, v)
	}
	smoothed, err := smoothVehicles(tranzyClient, filtered, filter)
	if err != nil {
		return nil, err
	}
	go ObserveVehicleSegmentTravelTimes(tranzyClient.Location(), smoothed)
	return smoothed, nil
}

func tripIDSet(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func smoothVehicles(tranzyClient *tranzy.Client, apiVehicles []models.Vehicle, filter VehicleFilter) ([]models.Vehicle, error) {
	dbVehicles, err := getVehiclesFromDB(filter)
	if err != nil {
		for i := range apiVehicles {
			if float64(apiVehicles[i].Speed) < MinSpeedFloor {
				apiVehicles[i].Speed = MinSpeedFloor
			}
		}
		return apiVehicles, nil
	}

	dbMap := make(map[int]models.Vehicle)
	for _, dbV := range dbVehicles {
		dbMap[dbV.ID] = dbV
	}

	apiMap := make(map[int]bool)
	for i, v := range apiVehicles {
		apiMap[v.ID] = true
		newSpeed := v.Speed
		if prev, exists := dbMap[v.ID]; exists {
			if v.Timestamp != prev.Timestamp {
				newSpeed = (float64(v.Speed) * EmaAlpha) + (float64(prev.Speed) * (1 - EmaAlpha))
			} else {
				newSpeed = prev.Speed
			}
		}
		if newSpeed < MinSpeedFloor {
			newSpeed = MinSpeedFloor
		}
		apiVehicles[i].Speed = newSpeed
	}

	gracePeriod := 1 * time.Minute
	now := time.Now().In(tranzyClient.Location())
	for _, dbV := range dbVehicles {
		if !apiMap[dbV.ID] {
			t, err := time.Parse(time.RFC3339, dbV.Timestamp)
			if err == nil && now.Sub(t) <= gracePeriod {
				apiVehicles = append(apiVehicles, dbV)
			}
		}
	}
	return apiVehicles, nil
}

func getVehiclesFromDB(filter VehicleFilter) ([]models.Vehicle, error) {
	var conditions []string
	var args []any
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
	return queryRows(`SELECT * FROM vehicles`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Vehicle, error) {
			var v models.Vehicle
			err := rows.Scan(&v.ID, &v.Label, &v.Latitude, &v.Longitude, &v.Timestamp, &v.VehicleType, &v.BikeAccessible, &v.WheelchairAccessible, &v.Speed, &v.RouteID, &v.TripID)
			return v, err
		})
}

func storeVehiclesInDB(vehicles []models.Vehicle) error {
	return batchExec(`
		INSERT OR REPLACE INTO vehicles
		(id, label, latitude, longitude, timestamp, vehicle_type, bike_accessible, wheelchair_accessible, speed, route_id, trip_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, v := range vehicles {
				if _, err := stmt.Exec(v.ID, v.Label, v.Latitude, v.Longitude, v.Timestamp, v.VehicleType, v.BikeAccessible, v.WheelchairAccessible, v.Speed, v.RouteID, v.TripID); err != nil {
					return fmt.Errorf("error inserting vehicle: %w", err)
				}
			}
			return nil
		})
}
