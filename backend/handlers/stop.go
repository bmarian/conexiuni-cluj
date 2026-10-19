package handlers

import (
	"conexiuni-cluj/models"
	"database/sql"
)

type StopFilter struct {
	StopID *int
}

func (city *City) GetStops(filter StopFilter) ([]models.Stop, error) {
	var conditions []string
	var args []any
	if filter.StopID != nil {
		conditions = append(conditions, "stop_id = ?")
		args = append(args, *filter.StopID)
	}
	return queryRows(city.DB, `SELECT stop_id, stop_name, stop_desc, stop_lat, stop_lon, location_type, stop_code, station_id FROM stops`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Stop, error) {
			var s models.Stop
			err := rows.Scan(&s.StopID, &s.StopName, &s.StopDesc, &s.StopLat, &s.StopLon, &s.LocationType, &s.StopCode, &s.StationID)
			// Stored before stations were grouped.
			if s.StationID == 0 {
				s.StationID = s.StopID
			}
			return s, err
		})
}
