package database

import (
	"context"
	"time"
)

// Rate-limit ANALYZE/optimize calls under concurrent cache writes.
func (s *Store) Optimize() error {
	s.optimizeMu.Lock()
	defer s.optimizeMu.Unlock()
	if time.Since(s.lastOptimize) < 10*time.Minute {
		return nil
	}
	s.lastOptimize = time.Now()
	if _, err := s.DB.Exec("PRAGMA optimize"); err != nil {
		return err
	}
	if _, err := s.DB.Exec("ANALYZE"); err != nil {
		return err
	}
	s.Logf("Database optimized")
	return nil
}

func (s *Store) Vacuum() error {
	if _, err := s.DB.Exec("VACUUM"); err != nil {
		return err
	}
	s.Logf("Database vacuumed")
	return nil
}

// Returns once ctx is done.
func (s *Store) RunVacuumScheduler(ctx context.Context) {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		s.Logf("Warning: could not load Europe/Bucharest timezone, falling back to UTC: %v", err)
		loc = time.UTC
	}
	for {
		now := time.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, loc)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		s.Logf("Next scheduled vacuum at %s", next.Format("2006-01-02 15:04:05"))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
		if err := s.Vacuum(); err != nil {
			s.Logf("Scheduled vacuum failed: %v", err)
		}
	}
}

// Size is the database file's size in bytes.
func (s *Store) Size() (int64, error) {
	var pages, pageSize int64
	if err := s.DB.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		return 0, err
	}
	if err := s.DB.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		return 0, err
	}
	return pages * pageSize, nil
}
