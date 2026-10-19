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
	return queryRows(city.DB, `SELECT * FROM stops`+whereClause(conditions), args,
		func(rows *sql.Rows) (models.Stop, error) {
			var s models.Stop
			err := rows.Scan(&s.StopID, &s.StopName, &s.StopDesc, &s.StopLat, &s.StopLon, &s.LocationType, &s.StopCode)
			return s, err
		})
}
