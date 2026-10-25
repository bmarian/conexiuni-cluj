// Package transit reads a city's network as the last import stored it: lines, stops, patterns, shapes, timetables and stop times.
package transit

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"strings"
)

type Data struct {
	store *database.Store
	db    *sql.DB
}

func New(store *database.Store) *Data { return &Data{store: store, db: store.DB} }

type RouteFilter struct {
	RouteID        *int
	RouteShortName *string
}

func (d *Data) Routes(filter RouteFilter) ([]models.Route, error) {
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
	return database.QueryRows(d.db, `SELECT * FROM routes`+database.WhereClause(conditions), args,
		func(rows *sql.Rows) (models.Route, error) {
			var r models.Route
			err := rows.Scan(&r.RouteID, &r.AgencyID, &r.RouteShortName, &r.RouteLongName, &r.RouteType, &r.RouteDesc, &r.RouteColor)
			return r, err
		})
}

type StopFilter struct {
	StopID *int
}

func (d *Data) Stops(filter StopFilter) ([]models.Stop, error) {
	var conditions []string
	var args []any
	if filter.StopID != nil {
		conditions = append(conditions, "stop_id = ?")
		args = append(args, *filter.StopID)
	}
	return database.QueryRows(d.db, `SELECT stop_id, stop_name, stop_desc, stop_lat, stop_lon, location_type, stop_code, station_id FROM stops`+database.WhereClause(conditions), args,
		func(rows *sql.Rows) (models.Stop, error) {
			var s models.Stop
			err := rows.Scan(&s.StopID, &s.StopName, &s.StopDesc, &s.StopLat, &s.StopLon, &s.LocationType, &s.StopCode, &s.StationID)
			if s.StationID == 0 {
				s.StationID = s.StopID
			}
			return s, err
		})
}

type TripFilter struct {
	RouteID *int
	TripID  *string
}

func (d *Data) Trips(filter TripFilter) ([]models.Trip, error) {
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
	return database.QueryRows(d.db, `SELECT * FROM trips`+database.WhereClause(conditions), args,
		func(rows *sql.Rows) (models.Trip, error) {
			var t models.Trip
			err := rows.Scan(&t.TripID, &t.RouteID, &t.DirectionID, &t.TripHeadsign, &t.BlockID, &t.ShapeID, &t.WheelchairAccessible, &t.BikesAllowed)
			return t, err
		})
}

func (d *Data) TripStops() ([]models.TripStop, error) {
	return database.QueryRows(d.db, `SELECT trip_id, stop_id, stop_sequence FROM trip_stops ORDER BY trip_id, stop_sequence`, nil,
		func(rows *sql.Rows) (models.TripStop, error) {
			var st models.TripStop
			err := rows.Scan(&st.TripID, &st.StopID, &st.StopSequence)
			return st, err
		})
}

// Every shape when ids is empty.
func (d *Data) Shapes(ids []string) ([]models.Shape, error) {
	var conditions []string
	var args []any
	if len(ids) > 0 {
		conditions = append(conditions, "shape_id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	return database.QueryRows(d.db, `SELECT * FROM shapes`+database.WhereClause(conditions)+` ORDER BY shape_id, shape_pt_sequence`, args,
		func(rows *sql.Rows) (models.Shape, error) {
			var s models.Shape
			err := rows.Scan(&s.ShapeID, &s.ShapePtLat, &s.ShapePtLon, &s.ShapePtSequence, &s.ShapeDistTraveled)
			return s, err
		})
}

// A line's direction that has a pattern.
type RouteDirection struct {
	RouteID        int
	DirectionID    int
	RouteShortName string
}

func (d *Data) RouteDirections() ([]RouteDirection, error) {
	return database.QueryRows(d.db, `
		SELECT DISTINCT r.route_id, t.direction_id, r.route_short_name
		FROM routes r
		INNER JOIN trips t ON t.route_id = r.route_id
		ORDER BY r.route_id, t.direction_id`, nil,
		func(rows *sql.Rows) (RouteDirection, error) {
			var rd RouteDirection
			err := rows.Scan(&rd.RouteID, &rd.DirectionID, &rd.RouteShortName)
			return rd, err
		})
}

// A line as a list names it.
type RouteLabel struct {
	ShortName string
	Color     string
}

func (d *Data) StopNames(ids []int) (map[int]string, error) {
	out := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := placeholders(ids)
	err := database.QueryEach(d.db, `SELECT stop_id, stop_name FROM stops WHERE stop_id IN (`+in+`)`, args, func(rows *sql.Rows) error {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		out[id] = name
		return nil
	})
	return out, err
}

func (d *Data) RouteLabels(ids []int) (map[int]RouteLabel, error) {
	out := make(map[int]RouteLabel, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := placeholders(ids)
	err := database.QueryEach(d.db, `SELECT route_id, route_short_name, COALESCE(route_color, '') FROM routes WHERE route_id IN (`+in+`)`, args, func(rows *sql.Rows) error {
		var id int
		var label RouteLabel
		if err := rows.Scan(&id, &label.ShortName, &label.Color); err != nil {
			return err
		}
		out[id] = label
		return nil
	})
	return out, err
}

func placeholders(ids []int) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}
