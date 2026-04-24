package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const ShapesCacheId = "SHAPES"

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
		func() ([]models.Shape, error) {
			shapes, err := tranzyFetch[[]models.Shape](tranzyClient, "/shapes")
			if err != nil {
				return nil, err
			}
			if shapes == nil {
				shapes = make([]models.Shape, 0)
			}
			return shapes, nil
		},
		storeShapesInDB,
		opts,
	)
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
	var conditions []string
	var args []any
	if filter.ShapeID != nil {
		conditions = append(conditions, "shape_id = ?")
		args = append(args, *filter.ShapeID)
	} else if len(filter.ShapeIDs) > 0 {
		ph := strings.Repeat("?,", len(filter.ShapeIDs))
		conditions = append(conditions, "shape_id IN ("+ph[:len(ph)-1]+")")
		for _, id := range filter.ShapeIDs {
			args = append(args, id)
		}
	}
	return queryRows(`SELECT * FROM shapes`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Shape, error) {
			var s models.Shape
			err := rows.Scan(&s.ShapeID, &s.ShapePtLat, &s.ShapePtLon, &s.ShapePtSequence, &s.ShapeDistTraveled)
			return s, err
		})
}

func storeShapesInDB(shapes []models.Shape) error {
	return batchExec(`
		INSERT OR REPLACE INTO shapes
		(shape_id, shape_pt_lat, shape_pt_lon, shape_pt_sequence, shape_dist_traveled)
		VALUES (?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, s := range shapes {
				if _, err := stmt.Exec(s.ShapeID, s.ShapePtLat, s.ShapePtLon, s.ShapePtSequence, s.ShapeDistTraveled); err != nil {
					return fmt.Errorf("error inserting shape: %w", err)
				}
			}
			return nil
		})
}
