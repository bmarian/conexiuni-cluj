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
