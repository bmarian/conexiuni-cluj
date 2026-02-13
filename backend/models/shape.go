package models

import "database/sql"

type Shape struct {
	ShapeID           string          `json:"shape_id" db:"shape_id"`
	ShapePtLat        float64         `json:"shape_pt_lat" db:"shape_pt_lat"`
	ShapePtLon        float64         `json:"shape_pt_lon" db:"shape_pt_lon"`
	ShapePtSequence   int             `json:"shape_pt_sequence" db:"shape_pt_sequence"`
	ShapeDistTraveled sql.NullFloat64 `json:"shape_dist_traveled,omitempty" db:"shape_dist_traveled"`
}
