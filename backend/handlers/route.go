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
	RoutesCacheId = "ROUTES"
)

type RouteFilter struct {
	RouteID        *int
	RouteShortName *string
}

func GetRoutes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter RouteFilter) ([]models.Route, error) {
	opts := CacheOpts[[]models.Route]{}

	if filter.RouteID != nil || filter.RouteShortName != nil {
		f := filter
		opts.PostProcess = func(rs []models.Route) []models.Route {
			var out []models.Route
			for _, r := range rs {
				if f.RouteID != nil && r.RouteID != *f.RouteID {
					continue
				}
				if f.RouteShortName != nil && r.RouteShortName != *f.RouteShortName {
					continue
				}
				out = append(out, r)
			}
			return out
		}
	} else {
		opts.Optimize = true
	}

	return HandleCached(RoutesCacheId, cacheShelfLife,
		func() ([]models.Route, error) { return getRoutesFromDB(filter) },
		func() ([]models.Route, error) { return requestRoutes(tranzyClient) },
		storeRoutesInDB,
		opts,
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

func getRoutesFromDB(filter RouteFilter) ([]models.Route, error) {
	query := `SELECT * FROM routes`
	var args []any
	var conditions []string

	if filter.RouteID != nil {
		conditions = append(conditions, "route_id = ?")
		args = append(args, *filter.RouteID)
	}
	if filter.RouteShortName != nil {
		conditions = append(conditions, "route_short_name = ?")
		args = append(args, *filter.RouteShortName)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := database.DB.Query(query, args...)
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
