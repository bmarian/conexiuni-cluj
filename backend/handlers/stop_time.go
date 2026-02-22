package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	StopTimesCacheId    = "STOP_TIMES"
	APIStopTimesCacheId = "API_STOP_TIMES"
)

type CacheTimes struct {
	ShapeCacheShelfLife       time.Duration
	RouteCacheShelfLife       time.Duration
	TripCacheShelfLife        time.Duration
	StopCacheShelfLife        time.Duration
	StopTimeCacheShelfLife    time.Duration
	APIStopTimeCacheShelfLife time.Duration
}

type StopTimeFilter struct {
	RouteShortName *string
}

func GetStopTimes(tranzyClient *tranzy.Client, filter StopTimeFilter, cacheTimes CacheTimes) ([]models.StopTime, error) {
	return requestStopTimes(tranzyClient, filter, cacheTimes)
}

func requestStopTimes(tranzyClient *tranzy.Client, filter StopTimeFilter, cacheTimes CacheTimes) ([]models.StopTime, error) {
	routes, errRoutes := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{RouteShortName: filter.RouteShortName})
	if errRoutes != nil || len(routes) == 0 {
		return nil, errRoutes
	}
	route := routes[0]

	trips, errTrips := GetTrips(tranzyClient, cacheTimes.TripCacheShelfLife, TripFilter{RouteID: &route.RouteID})
	if errTrips != nil || len(trips) == 0 {
		return nil, errTrips
	}

	apiStopTimes, err := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife, APIStopTimeFilter{})
	if err != nil {
		return nil, err
	}

	groupedRaw := make(map[string][]models.RequestStopTime)
	count := 0
	for _, t := range trips {
		tripID := t.TripID
		for _, ast := range apiStopTimes {
			if ast.TripID == tripID {
				groupedRaw[tripID] = append(groupedRaw[tripID], ast)
				count++
			}
		}
		sort.Slice(groupedRaw[tripID], func(i, j int) bool {
			return groupedRaw[tripID][i].StopSequence < groupedRaw[tripID][j].StopSequence
		})
	}

	out := make([]models.StopTime, 0, count)
	for _, gr := range groupedRaw {
		for _, st := range gr {

			stops, errStops := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{StopID: &st.StopID})
			stopHeadsign := ""
			if errStops == nil && len(stops) != 0 {
				stop := stops[0]
				stopHeadsign = stop.StopName
			}

			out = append(out, models.StopTime{
				TripID:       st.TripID,
				StopID:       st.StopID,
				StopSequence: st.StopSequence,
				StopHeadsign: stopHeadsign,
			})
		}
	}

	return out, nil
}

type APIStopTimeFilter struct {
	TripID *string
}

func getAPIStopTimes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter APIStopTimeFilter) ([]models.RequestStopTime, error) {
	opts := CacheOpts[[]models.RequestStopTime]{}

	if filter.TripID != nil {
		f := filter
		opts.PostProcess = func(ts []models.RequestStopTime) []models.RequestStopTime {
			var out []models.RequestStopTime
			for _, t := range ts {
				if f.TripID != nil && t.TripID != *f.TripID {
					continue
				}
				out = append(out, t)
			}
			return out
		}
	} else {
		opts.Optimize = true
	}

	return HandleCached(APIStopTimesCacheId, cacheShelfLife,
		func() ([]models.RequestStopTime, error) { return getAPIStopTimesFromDB(filter) },
		func() ([]models.RequestStopTime, error) { return requestAPIStopTimes(tranzyClient) },
		storeAPIStopTimesInDB,
		opts,
	)
}

func requestAPIStopTimes(tranzyClient *tranzy.Client) ([]models.RequestStopTime, error) {
	data, err := tranzyClient.DoRequest("/stop_times", nil)
	if err != nil {
		return nil, err
	}

	var raw []models.RequestStopTime
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api stop times: %w", err)
	}

	return raw, nil
}

func getAPIStopTimesFromDB(filter APIStopTimeFilter) ([]models.RequestStopTime, error) {
	query := `SELECT * FROM api_stop_times`
	var args []any
	var conditions []string

	if filter.TripID != nil {
		conditions = append(conditions, "trip_id = ?")
		args = append(args, *filter.TripID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying api stop times: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var stopTimes []models.RequestStopTime
	for rows.Next() {
		var st models.RequestStopTime
		if err := rows.Scan(&st.TripID, &st.StopID, &st.StopSequence); err != nil {
			return nil, fmt.Errorf("error scanning api stop time: %w", err)
		}
		stopTimes = append(stopTimes, st)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading api stop times: %w", err)
	}

	return stopTimes, nil
}

func storeAPIStopTimesInDB(stopTimes []models.RequestStopTime) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO api_stop_times (trip_id, stop_id, stop_sequence)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, st := range stopTimes {
		if _, err := stmt.Exec(st.TripID, st.StopID, st.StopSequence); err != nil {
			return fmt.Errorf("error inserting api stop time: %w", err)
		}
	}

	return nil
}

/*
All data for trip_id: 1_0

stop_times by trip_id 1_0:
  {
    "trip_id": "1_0",
    "stop_id": 1,
    "stop_sequence": 0
  },
  {
    "trip_id": "1_0",
    "stop_id": 2,
    "stop_sequence": 1
  },
  ...

trip of trip_id:
  {
    "route_id": 1,
    "trip_id": "1_0",
    "trip_headsign": "P-ta 1 Mai Sos",
    "direction_id": 0,
    "block_id": 1,
    "shape_id": "1_0"
  }

stop of stop_id:
  {
    "stop_id": 1,
    "stop_name": "Disp. Clăbucet",
    "stop_lat": 46.75144,
    "stop_lon": 23.54292,
    "location_type": 0,
    "stop_code": ""
  },

route of route_id:
  {
    "agency_id": 2,
    "route_id": 1,
    "route_short_name": "1",
    "route_long_name": "Str. Bucium - P-ta 1 Mai",
    "route_color": "#f3513c",
    "route_type": 11,
    "route_desc": "Str. Bucium - P-ta 1 Mai"
  },

shapes of shape_id:
  [
    {
      "shape_id": "1_0",
      "shape_pt_lat": 46.75123,
      "shape_pt_lon": 23.54317,
      "shape_pt_sequence": 0
    },
    {
      "shape_id": "1_0",
      "shape_pt_lat": 46.75125,
      "shape_pt_lon": 23.5432,
      "shape_pt_sequence": 1
    },
    {
      "shape_id": "1_0",
      "shape_pt_lat": 46.75129,
      "shape_pt_lon": 23.54327,
      "shape_pt_sequence": 2
    },
    ...
  ]
*/
