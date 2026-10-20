package database

import "time"

// FetchedAt is when source was last read; zero when never.
func (s *Store) FetchedAt(source string) time.Time {
	var at int64
	if err := s.DB.QueryRow(`SELECT fetched_at FROM ingest_state WHERE source = ?`, source).Scan(&at); err != nil {
		return time.Time{}
	}
	return time.Unix(at, 0)
}

func (s *Store) MarkFetched(source string, at time.Time) {
	if _, err := s.DB.Exec(`INSERT OR REPLACE INTO ingest_state (source, fetched_at) VALUES (?, ?)`, source, at.Unix()); err != nil {
		s.Logf("ingest: mark %s: %v", source, err)
	}
}

// ForgetFetches makes every source due again.
func (s *Store) ForgetFetches() error {
	_, err := s.DB.Exec(`DELETE FROM ingest_state`)
	return err
}
