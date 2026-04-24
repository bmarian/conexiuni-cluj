package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"fmt"
	"time"
)

const RoutesCacheId = "ROUTES"

type RouteFilter struct {
	RouteID        *int
	RouteShortName *string
}

func GetRoutes(tranzyClient *tranzy.Client, cacheShelfLife time.Duration, filter RouteFilter) ([]models.Route, error) {
	opts := CacheOpts[[]models.Route]{}
	if filter.RouteID != nil || filter.RouteShortName != nil {
		f := filter
		opts.PostProcess = func(rs []models.Route) []models.Route {
			out := make([]models.Route, 0)
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
		func() ([]models.Route, error) { return tranzyFetch[[]models.Route](tranzyClient, "/routes") },
		storeRoutesInDB,
		opts,
	)
}

func getRoutesFromDB(filter RouteFilter) ([]models.Route, error) {
	var conditions []string
	var args []any
	if filter.RouteID != nil {
		conditions = append(conditions, "route_id = ?")
		args = append(args, *filter.RouteID)
	}
	if filter.RouteShortName != nil {
		conditions = append(conditions, "route_short_name = ?")
		args = append(args, *filter.RouteShortName)
	}
	return queryRows(`SELECT * FROM routes`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Route, error) {
			var r models.Route
			err := rows.Scan(&r.RouteID, &r.AgencyID, &r.RouteShortName, &r.RouteLongName, &r.RouteType, &r.RouteDesc, &r.RouteColor)
			return r, err
		})
}

func storeRoutesInDB(routes []models.Route) error {
	return batchExec(`
		INSERT OR REPLACE INTO routes
		(route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, r := range routes {
				if _, err := stmt.Exec(r.RouteID, r.AgencyID, r.RouteShortName, r.RouteLongName, r.RouteType, r.RouteDesc, models.ResolveRouteDisplayColor(r.RouteShortName)); err != nil {
					return fmt.Errorf("error inserting route: %w", err)
				}
			}
			return nil
		})
}
