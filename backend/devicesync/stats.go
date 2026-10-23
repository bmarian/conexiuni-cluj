package devicesync

// Stats is what the admin page shows: counts and size, nothing a record holds.
type Stats struct {
	Chains  int            `json:"chains"`
	Devices map[string]int `json:"devices"`
	DBBytes int64          `json:"db_bytes"`
}

func (s *Service) Stats() (Stats, error) {
	st := Stats{Devices: map[string]int{}}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM chains`).Scan(&st.Chains); err != nil {
		return st, err
	}
	rows, err := s.db.Query(`SELECT kind, COUNT(*) FROM devices GROUP BY kind`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return st, err
		}
		st.Devices[kind] = n
	}
	if err := rows.Err(); err != nil {
		return st, err
	}
	st.DBBytes, err = s.size()
	return st, err
}
