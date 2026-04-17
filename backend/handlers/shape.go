package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	ShapesCacheId = "SHAPES"
)

type ShapeFilter struct {
	ShapeID  *string
	ShapeIDs []string
}

func GetShapes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter ShapeFilter) ([]models.Shape, error) {
	opts := CacheOpts[[]models.Shape]{}

	if filter.ShapeID != nil || len(filter.ShapeIDs) > 0 {
		f := filter
		idSet := shapeIDSet(f.ShapeIDs)
		opts.PostProcess = func(ss []models.Shape) []models.Shape {
			out := make([]models.Shape, 0)
			for _, s := range ss {
				if f.ShapeID != nil && s.ShapeID != *f.ShapeID {
					continue
				}
				if idSet != nil {
					if _, ok := idSet[s.ShapeID]; !ok {
						continue
					}
				}
				out = append(out, s)
			}
			return out
		}
	} else {
		opts.Optimize = true
	}

	return HandleCached(ShapesCacheId, cacheShelfLife,
		func() ([]models.Shape, error) { return getShapesFromDB(filter) },
		func() ([]models.Shape, error) { return requestShapes(tranzyClient) },
		storeShapesInDB,
		opts,
	)
}

func requestShapes(tranzyClient *tranzy.Client) ([]models.Shape, error) {
	data, err := tranzyClient.DoRequest("/shapes", nil)
	if err != nil {
		return nil, err
	}

	var shapes []models.Shape
	if err := json.Unmarshal(data, &shapes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shapes: %w", err)
	}
	if shapes == nil {
		shapes = make([]models.Shape, 0)
	}

	return shapes, nil
}

func shapeIDSet(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func getShapesFromDB(filter ShapeFilter) ([]models.Shape, error) {
	query := `SELECT * FROM shapes`
	var args []any
	var conditions []string

	if filter.ShapeID != nil {
		conditions = append(conditions, "shape_id = ?")
		args = append(args, *filter.ShapeID)
	} else if len(filter.ShapeIDs) > 0 {
		placeholders := strings.Repeat("?,", len(filter.ShapeIDs))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, "shape_id IN ("+placeholders+")")
		for _, id := range filter.ShapeIDs {
			args = append(args, id)
		}
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying shapes: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	shapes := make([]models.Shape, 0)
	for rows.Next() {
		var shape models.Shape
		err := rows.Scan(
			&shape.ShapeID,
			&shape.ShapePtLat,
			&shape.ShapePtLon,
			&shape.ShapePtSequence,
			&shape.ShapeDistTraveled,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning shape: %w", err)
		}
		shapes = append(shapes, shape)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading shapes: %w", err)
	}

	return shapes, nil
}

func storeShapesInDB(shapes []models.Shape) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO shapes
		(shape_id, shape_pt_lat, shape_pt_lon, shape_pt_sequence, shape_dist_traveled)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, shape := range shapes {
		if _, err := stmt.Exec(
			shape.ShapeID,
			shape.ShapePtLat,
			shape.ShapePtLon,
			shape.ShapePtSequence,
			shape.ShapeDistTraveled,
		); err != nil {
			return fmt.Errorf("error inserting shape: %w", err)
		}
	}

	return nil
}
