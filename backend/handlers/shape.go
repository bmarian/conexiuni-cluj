package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	ShapesCacheId = "SHAPES"
)

func GetShapes(c fiber.Ctx, tranzyClient *tranzy.Client, cacheShelfLife time.Duration) error {
	return HandleCached(c, ShapesCacheId, cacheShelfLife,
		getShapesFromDB,
		func() ([]models.Shape, error) { return requestShapes(tranzyClient) },
		storeShapesInDB,
		CacheOpts[[]models.Shape]{Optimize: true},
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

	return shapes, nil
}

func getShapesFromDB() ([]models.Shape, error) {
	rows, err := database.DB.Query(`SELECT * FROM shapes`)
	if err != nil {
		return nil, fmt.Errorf("error querying shapes: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var shapes []models.Shape
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
