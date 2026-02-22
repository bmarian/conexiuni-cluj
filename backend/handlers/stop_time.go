package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	StopTimesCacheId    = "STOP_TIMES"
	APIStopTimesCacheId = "API_STOP_TIMES"
)

type CacheTimes struct {
	VehicleCacheShelfLife     time.Duration
	ShapeCacheShelfLife       time.Duration
	RouteCacheShelfLife       time.Duration
	TripCacheShelfLife        time.Duration
	StopCacheShelfLife        time.Duration
	TimetableCacheShelfLife   time.Duration
	StopTimeCacheShelfLife    time.Duration
	APIStopTimeCacheShelfLife time.Duration
}

type StopTimeFilter struct {
	RouteShortName *string
}

func GetStopTimes(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, filter StopTimeFilter, cacheTimes CacheTimes) ([]models.StopTime, error) {
	return requestStopTimes(tranzyClient, ctpCjClient, filter, cacheTimes)
}

func requestStopTimes(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, filter StopTimeFilter, cacheTimes CacheTimes) ([]models.StopTime, error) {
	raw, err := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife)
	if err != nil {
		return nil, err
	}

	groupedRaw := make(map[string][]models.RequestStopTime)
	for _, r := range raw {
		groupedRaw[r.TripID] = append(groupedRaw[r.TripID], r)
	}

	count := 0
	out := make([]models.StopTime, len(raw))
	for _, group := range groupedRaw {
		for _, r := range group {
			trips, errTrip := GetTrips(tranzyClient, cacheTimes.TripCacheShelfLife, TripFilter{TripID: &r.TripID})
			if errTrip != nil || len(trips) == 0 {
				continue
			}
			trip := trips[0]

			stops, errStops := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{StopID: &r.StopID})
			if errStops != nil || len(stops) == 0 {
				continue
			}
			stop := stops[0]
			stopHeadsign := stop.StopName

			routes, errRoutes := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{RouteID: &trip.RouteID})
			if errRoutes != nil || len(routes) == 0 {
				continue
			}
			route := routes[0]

			shapes, errShapes := GetShapes(tranzyClient, cacheTimes.ShapeCacheShelfLife, ShapeFilter{ShapeID: &trip.ShapeID})
			if errShapes != nil || len(shapes) == 0 {
				continue
			}

			_, errTimetable := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, route.RouteShortName)

			if errTimetable != nil {
				continue
			}

			out[count] = models.StopTime{
				TripID:       r.TripID,
				StopID:       r.StopID,
				StopSequence: r.StopSequence,
				StopHeadsign: stopHeadsign,
			}
			count++
		}
	}
	return out, nil
}

func getAPIStopTimes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration) ([]models.RequestStopTime, error) {
	return HandleCached(APIStopTimesCacheId, cacheShelfLife,
		getAPIStopTimesFromDB,
		func() ([]models.RequestStopTime, error) { return requestAPIStopTimes(tranzyClient) },
		storeAPIStopTimesInDB,
		CacheOpts[[]models.RequestStopTime]{Optimize: true},
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

func getAPIStopTimesFromDB() ([]models.RequestStopTime, error) {
	rows, err := database.DB.Query(`SELECT * FROM api_stop_times`)
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
