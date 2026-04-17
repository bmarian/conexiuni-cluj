package models

import "encoding/json"

type LocationType int

const (
	StopOrPlatform LocationType = 0
	Station        LocationType = 1
	EntranceExit   LocationType = 2
	GenericNode    LocationType = 3
	BoardingArea   LocationType = 4
)

type Stop struct {
	StopID       int          `json:"stop_id" db:"stop_id"`
	StopName     string       `json:"stop_name" db:"stop_name"`
	StopDesc     string       `json:"stop_desc" db:"stop_desc"`
	StopLat      float64      `json:"stop_lat" db:"stop_lat"`
	StopLon      float64      `json:"stop_lon" db:"stop_lon"`
	LocationType LocationType `json:"location_type" db:"location_type"`
	StopCode     string       `json:"stop_code" db:"stop_code"`
}

func (s *Stop) UnmarshalJSON(data []byte) error {
	var raw struct {
		StopID       int           `json:"stop_id"`
		StopName     string        `json:"stop_name"`
		StopDesc     *string       `json:"stop_desc"`
		StopLat      float64       `json:"stop_lat"`
		StopLon      float64       `json:"stop_lon"`
		LocationType *LocationType `json:"location_type"`
		StopCode     *string       `json:"stop_code"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	s.StopID = raw.StopID
	s.StopName = raw.StopName
	if raw.StopDesc != nil {
		s.StopDesc = *raw.StopDesc
	} else {
		s.StopDesc = ""
	}
	s.StopLat = raw.StopLat
	s.StopLon = raw.StopLon
	if raw.LocationType != nil {
		s.LocationType = *raw.LocationType
	} else {
		s.LocationType = StopOrPlatform
	}
	if raw.StopCode != nil {
		s.StopCode = *raw.StopCode
	} else {
		s.StopCode = ""
	}

	return nil
}
