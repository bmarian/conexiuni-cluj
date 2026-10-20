package updates

import (
	"conexiuni-cluj/database"
	"database/sql"
	"encoding/json"
	"slices"
	"time"
)

// Brief is a recorded change in a line: which line, when, and what kinds of change.
type Brief struct {
	Line       string
	Color      string
	DetectedAt int64
	Kinds      []string
}

// Briefs are the latest changes since since, newest first.
func (s *Service) Briefs(since time.Time, limit int) ([]Brief, error) {
	return database.QueryRows(s.store.DB, `
		SELECT c.route_short_name, COALESCE(r.route_color, ''), c.detected_at, c.changes
		FROM route_changes c
		LEFT JOIN (SELECT route_short_name, MAX(route_color) AS route_color FROM routes GROUP BY route_short_name) r
		  ON r.route_short_name = c.route_short_name
		WHERE c.detected_at >= ?
		ORDER BY c.detected_at DESC
		LIMIT ?`,
		[]any{since.UnixMilli(), limit},
		func(rows *sql.Rows) (Brief, error) {
			b := Brief{Kinds: []string{}}
			var payload string
			if err := rows.Scan(&b.Line, &b.Color, &b.DetectedAt, &payload); err != nil {
				return b, err
			}
			var items []RouteChangeItem
			if err := json.Unmarshal([]byte(payload), &items); err != nil {
				return b, err
			}
			for _, it := range items {
				if !slices.Contains(b.Kinds, it.Kind) {
					b.Kinds = append(b.Kinds, it.Kind)
				}
			}
			return b, nil
		})
}
