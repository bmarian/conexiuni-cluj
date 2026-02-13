package tranzy

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
	"time"
)

// Cache configuration constants
const (
	RoutesCacheID  = "routes"
	RouteCacheLife = 24 * time.Hour
)

func (c *Client) GetRoutes() ([]models.Route, error) {
	// Check if cache is still valid
	isCacheValid := database.IsCacheValid(RoutesCacheID)

	// Return from database if cache is valid
	if isCacheValid {
		return getRoutesFromDB()
	}

	// Fetch from API if cache is invalid
	routes, err := c.fetchRoutesFromAPI()
	if err != nil {
		return nil, err
	}

	// Store in database
	if err := storeRoutesInDB(routes); err != nil {
		return nil, fmt.Errorf("error storing routes in database: %w", err)
	}

	// Update cache timestamp
	_ = database.UpdateCache(RoutesCacheID, int64(RouteCacheLife))

	return routes, nil
}

// fetchRoutesFromAPI retrieves routes from the Tranzy API
func (c *Client) fetchRoutesFromAPI() ([]models.Route, error) {
	data, err := c.DoRequest("/routes", nil)
	if err != nil {
		return nil, err
	}

	var routes []models.Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal routes: %w", err)
	}

	return routes, nil
}

// getRoutesFromDB retrieves all routes from the database
func getRoutesFromDB() ([]models.Route, error) {
	rows, err := database.DB.Query(`SELECT route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color FROM routes`)
	if err != nil {
		return nil, fmt.Errorf("error querying routes: %w", err)
	}
	defer rows.Close()

	var routes []models.Route
	for rows.Next() {
		var route models.Route
		if err := rows.Scan(
			&route.RouteID,
			&route.AgencyID,
			&route.RouteShortName,
			&route.RouteLongName,
			&route.RouteType,
			&route.RouteDesc,
			&route.RouteColor,
		); err != nil {
			return nil, fmt.Errorf("error scanning route: %w", err)
		}
		routes = append(routes, route)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading routes: %w", err)
	}

	return routes, nil
}

// storeRoutesInDB stores routes in the database, replacing existing ones
func storeRoutesInDB(routes []models.Route) error {
	stmt, err := database.DB.Prepare(`
		INSERT OR REPLACE INTO routes 
		(route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

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
