package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"fmt"
	"time"
)

const (
	StopTimesCacheId = "STOP_TIMES"
)

func GetStopTimes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration) ([]models.StopTime, error) {
	return requestStopTimes(tranzyClient)
}

func requestStopTimes(tranzyClient *tranzy.Client) ([]models.StopTime, error) {
	data, err := tranzyClient.DoRequest("/stop_times", nil)
	if err != nil {
		return nil, err
	}

	var raw []models.RequestStopTime
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stop times: %w", err)
	}

	out := make([]models.StopTime, len(raw))
	for i, r := range raw {
		out[i] = models.StopTime{
			TripID:       r.TripID,
			StopID:       r.StopID,
			StopSequence: r.StopSequence,
		}
	}
	return out, nil
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
