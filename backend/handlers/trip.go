package handlers

import (
	"conexiuni-cluj/models"
	"database/sql"
)

type TripFilter struct {
	RouteID *int
	TripID  *string
}

func (city *City) GetTrips(filter TripFilter) ([]models.Trip, error) {
	var conditions []string
	var args []any
	if filter.RouteID != nil {
		conditions = append(conditions, "route_id = ?")
		args = append(args, *filter.RouteID)
	}
	if filter.TripID != nil {
		conditions = append(conditions, "trip_id = ?")
		args = append(args, *filter.TripID)
	}
	return queryRows(city.DB, `SELECT * FROM trips`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Trip, error) {
			var t models.Trip
			err := rows.Scan(&t.TripID, &t.RouteID, &t.DirectionID, &t.TripHeadsign, &t.BlockID, &t.ShapeID, &t.WheelchairAccessible, &t.BikesAllowed)
			return t, err
		})
}

func (city *City) getTripStops() ([]models.TripStop, error) {
	return queryRows(city.DB, `SELECT trip_id, stop_id, stop_sequence FROM trip_stops ORDER BY trip_id, stop_sequence`, nil,
		func(rows *sql.Rows) (models.TripStop, error) {
			var st models.TripStop
			err := rows.Scan(&st.TripID, &st.StopID, &st.StopSequence)
			return st, err
		})
}
