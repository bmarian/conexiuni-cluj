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
	RoutesCacheId       = "ROUTES"
	RouteCacheShelfLife = 24 * time.Hour
)

func GetRoutes(c fiber.Ctx, tranzyClient *tranzy.Client) error {
	return HandleCachedData(
		c,
		RoutesCacheId,
		RouteCacheShelfLife,
		getRoutesFromDB,
		func() ([]models.Route, error) { return requestRoutes(tranzyClient) },
		storeRoutesInDB,
	)
}

func requestRoutes(tranzyClient *tranzy.Client) ([]models.Route, error) {
	data, err := tranzyClient.DoRequest("/routes", nil)
	if err != nil {
		return nil, err
	}

	var routes []models.Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal routes: %w", err)
	}

	return routes, nil
}

func getRoutesFromDB() ([]models.Route, error) {
	rows, err := database.DB.Query(`SELECT * FROM routes`)
	if err != nil {
		return nil, fmt.Errorf("error querying routes: %w", err)
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var routes []models.Route
	for rows.Next() {
		var route models.Route
		err := rows.Scan(
			&route.RouteID,
			&route.AgencyID,
			&route.RouteShortName,
			&route.RouteLongName,
			&route.RouteType,
			&route.RouteDesc,
			&route.RouteColor,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning route: %w", err)
		}
		routes = append(routes, route)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading routes: %w", err)
	}

	return routes, nil
}

func storeRoutesInDB(routes []models.Route) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO routes
		(route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}

	defer func(stmt *sql.Stmt) {
		_ = stmt.Close()
	}(stmt)

	for _, route := range routes {
		if _, err := stmt.Exec(
			route.RouteID,
			route.AgencyID,
			route.RouteShortName,
			route.RouteLongName,
			route.RouteType,
			route.RouteDesc,
			route.RouteColor,
		); err != nil {
			return fmt.Errorf("error inserting route: %w", err)
		}
	}

	return nil
}
