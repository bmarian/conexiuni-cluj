package models

import "database/sql"

// ShapeDB represents a shape as stored in the database and received from the API
type ShapeDB struct {
	ShapeID           string          `json:"shape_id" db:"shape_id"`
	ShapePtLat        float64         `json:"shape_pt_lat" db:"shape_pt_lat"`
	ShapePtLon        float64         `json:"shape_pt_lon" db:"shape_pt_lon"`
	ShapePtSequence   int             `json:"shape_pt_sequence" db:"shape_pt_sequence"`
	ShapeDistTraveled sql.NullFloat64 `json:"shape_dist_traveled,omitempty" db:"shape_dist_traveled"`
}

type Shape struct {
	ShapeID           string  `json:"shape_id"`
	ShapePtLat        float64 `json:"shape_pt_lat"`
	ShapePtLon        float64 `json:"shape_pt_lon"`
	ShapePtSequence   int     `json:"shape_pt_sequence"`
	ShapeDistTraveled float64 `json:"shape_dist_traveled"`
}

func (s *ShapeDB) Normalize() Shape {
	distTraveled := -1.0
	if s.ShapeDistTraveled.Valid {
		distTraveled = s.ShapeDistTraveled.Float64
	}

	return Shape{
		ShapeID:           s.ShapeID,
		ShapePtLat:        s.ShapePtLat,
		ShapePtLon:        s.ShapePtLon,
		ShapePtSequence:   s.ShapePtSequence,
		ShapeDistTraveled: distTraveled,
	}
}
