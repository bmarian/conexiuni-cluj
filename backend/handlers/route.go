package handlers

import (
	"conexiuni-cluj/models"
	"database/sql"
)

type RouteFilter struct {
	RouteID        *int
	RouteShortName *string
}

func (city *City) GetRoutes(filter RouteFilter) ([]models.Route, error) {
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
	return queryRows(city.DB, `SELECT * FROM routes`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Route, error) {
			var r models.Route
			err := rows.Scan(&r.RouteID, &r.AgencyID, &r.RouteShortName, &r.RouteLongName, &r.RouteType, &r.RouteDesc, &r.RouteColor)
			return r, err
		})
}
