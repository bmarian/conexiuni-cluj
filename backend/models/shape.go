package models

import "encoding/json"

type Shape struct {
	ShapeID           string  `json:"shape_id" db:"shape_id"`
	ShapePtLat        float64 `json:"shape_pt_lat" db:"shape_pt_lat"`
	ShapePtLon        float64 `json:"shape_pt_lon" db:"shape_pt_lon"`
	ShapePtSequence   int     `json:"shape_pt_sequence" db:"shape_pt_sequence"`
	ShapeDistTraveled float64 `json:"shape_dist_traveled" db:"shape_dist_traveled"`
}

func (s *Shape) UnmarshalJSON(data []byte) error {
	var raw struct {
		ShapeID           string   `json:"shape_id"`
		ShapePtLat        float64  `json:"shape_pt_lat"`
		ShapePtLon        float64  `json:"shape_pt_lon"`
		ShapePtSequence   int      `json:"shape_pt_sequence"`
		ShapeDistTraveled *float64 `json:"shape_dist_traveled"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	s.ShapeID = raw.ShapeID
	s.ShapePtLat = raw.ShapePtLat
	s.ShapePtLon = raw.ShapePtLon
	s.ShapePtSequence = raw.ShapePtSequence
	if raw.ShapeDistTraveled != nil {
		s.ShapeDistTraveled = *raw.ShapeDistTraveled
	} else {
		s.ShapeDistTraveled = -1.0
	}

	return nil
}
