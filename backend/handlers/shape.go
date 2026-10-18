package handlers

import (
	"conexiuni-cluj/models"
	"database/sql"
	"strings"
)

// GetShapes returns the points of the given shapes, or of every shape when ids is empty, in sequence order.
func (city *City) GetShapes(ids []string) ([]models.Shape, error) {
	var conditions []string
	var args []any
	if len(ids) > 0 {
		conditions = append(conditions, "shape_id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	return queryRows(city.DB, `SELECT * FROM shapes`+whereClause(conditions)+` ORDER BY shape_id, shape_pt_sequence`, args,
		func(rows *sql.Rows) (models.Shape, error) {
			var s models.Shape
			err := rows.Scan(&s.ShapeID, &s.ShapePtLat, &s.ShapePtLon, &s.ShapePtSequence, &s.ShapeDistTraveled)
			return s, err
		})
}
