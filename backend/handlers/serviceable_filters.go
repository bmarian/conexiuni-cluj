package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"encoding/json"
	"fmt"
)

type serviceabilitySets struct {
	hasStopInfoRows  bool
	stopIDsWithBuses map[int]struct{}
	usableRouteShort map[string]struct{}
}

func loadServiceabilitySetsFromStopInfo() (serviceabilitySets, error) {
	rows, err := database.DB.Query(`SELECT stop_id, trip_ids, shapes_short_name FROM stop_info`)
	if err != nil {
		return serviceabilitySets{}, fmt.Errorf("error querying stop_info for filtering: %w", err)
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	sets := serviceabilitySets{
		stopIDsWithBuses: make(map[int]struct{}),
		usableRouteShort: make(map[string]struct{}),
	}
	stopInfoRows := 0

	for rows.Next() {
		stopInfoRows++

		var (
			stopID         int
			tripIDsJSON    string
			shapeNamesJSON string
		)
		if err := rows.Scan(&stopID, &tripIDsJSON, &shapeNamesJSON); err != nil {
			return serviceabilitySets{}, fmt.Errorf("error scanning stop_info for filtering: %w", err)
		}

		var tripIDs []string
		if err := json.Unmarshal([]byte(tripIDsJSON), &tripIDs); err != nil {
			return serviceabilitySets{}, fmt.Errorf("error parsing stop_info.trip_ids for stop %d: %w", stopID, err)
		}
		if len(tripIDs) > 0 {
			sets.stopIDsWithBuses[stopID] = struct{}{}
		}

		var shapeNames []string
		if err := json.Unmarshal([]byte(shapeNamesJSON), &shapeNames); err != nil {
			return serviceabilitySets{}, fmt.Errorf("error parsing stop_info.shapes_short_name for stop %d: %w", stopID, err)
		}
		for _, shortName := range shapeNames {
			if shortName == "" {
				continue
			}
			sets.usableRouteShort[shortName] = struct{}{}
		}
	}

	if err := rows.Err(); err != nil {
		return serviceabilitySets{}, fmt.Errorf("error reading stop_info for filtering: %w", err)
	}
	if stopInfoRows == 0 {
		return sets, nil
	}

	var totalStops int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stops`).Scan(&totalStops); err != nil {
		return serviceabilitySets{}, fmt.Errorf("error counting stops for filtering: %w", err)
	}
	// Only enforce filtering after full stop_info coverage, to avoid partial
	// lists while warmup is still populating entries.
	if totalStops > 0 && stopInfoRows < totalStops {
		return serviceabilitySets{
			hasStopInfoRows: false,
		}, nil
	}
	sets.hasStopInfoRows = true

	return sets, nil
}

func FilterServiceableStops(stops []models.Stop) ([]models.Stop, error) {
	sets, err := loadServiceabilitySetsFromStopInfo()
	if err != nil {
		return nil, err
	}
	if !sets.hasStopInfoRows {
		return stops, nil
	}

	out := make([]models.Stop, 0, len(stops))
	for _, stop := range stops {
		if _, ok := sets.stopIDsWithBuses[stop.StopID]; ok {
			out = append(out, stop)
		}
	}
	return out, nil
}

func FilterServiceableRoutes(routes []models.Route) ([]models.Route, error) {
	sets, err := loadServiceabilitySetsFromStopInfo()
	if err != nil {
		return nil, err
	}
	if !sets.hasStopInfoRows {
		return routes, nil
	}

	out := make([]models.Route, 0, len(routes))
	for _, route := range routes {
		if _, ok := sets.usableRouteShort[route.RouteShortName]; ok {
			out = append(out, route)
		}
	}
	return out, nil
}
