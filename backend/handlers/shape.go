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
	ShapesCacheId       = "SHAPES"
	ShapeCacheShelfLife = 24 * time.Hour
)

func GetShapes(c fiber.Ctx, tranzyClient *tranzy.Client) error {
	return HandleCachedData(
		c,
		ShapesCacheId,
		ShapeCacheShelfLife,
		getShapesFromDB,
		func() ([]models.Shape, error) { return requestShapes(tranzyClient) },
		storeShapesInDB,
	)
}

func requestShapes(tranzyClient *tranzy.Client) ([]models.Shape, error) {
	data, err := tranzyClient.DoRequest("/shapes", nil)
	if err != nil {
		return nil, err
	}

	var shapesDB []models.ShapeDB
	if err := json.Unmarshal(data, &shapesDB); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shapes: %w", err)
	}

	shapes := make([]models.Shape, len(shapesDB))
	for i, shapeDB := range shapesDB {
		shapes[i] = shapeDB.Normalize()
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
		var shapeDB models.ShapeDB
		err := rows.Scan(
			&shapeDB.ShapeID,
			&shapeDB.ShapePtLat,
			&shapeDB.ShapePtLon,
			&shapeDB.ShapePtSequence,
			&shapeDB.ShapeDistTraveled,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning shape: %w", err)
		}
		shapes = append(shapes, shapeDB.Normalize())
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
		distTraveled := sql.NullFloat64{Valid: false}
		if shape.ShapeDistTraveled != -1 {
			distTraveled = sql.NullFloat64{Float64: shape.ShapeDistTraveled, Valid: true}
		}

		if _, err := stmt.Exec(
			shape.ShapeID,
			shape.ShapePtLat,
			shape.ShapePtLon,
			shape.ShapePtSequence,
			distTraveled,
		); err != nil {
			return fmt.Errorf("error inserting shape: %w", err)
		}
	}

	return nil
}
