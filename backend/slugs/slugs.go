// Package slugs gives stations and lines readable addresses that keep pointing at them.
package slugs

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"database/sql"
	"sync/atomic"
)

type Kind string

const (
	Station Kind = "station"
	Line    Kind = "line"
)

type row struct {
	kind    Kind
	slug    string
	id      int
	base    string
	current bool
}

type Index struct {
	current map[Kind]map[int]string
	owner   map[Kind]map[string]int
}

func (x *Index) Of(kind Kind, id int) string { return x.current[kind][id] }

func (x *Index) Find(kind Kind, slug string) (int, bool) {
	id, ok := x.owner[kind][slug]
	return id, ok
}

func Empty() *Index { return indexOf(nil) }

func indexOf(rows []row) *Index {
	x := &Index{current: map[Kind]map[int]string{}, owner: map[Kind]map[string]int{}}
	for _, k := range []Kind{Station, Line} {
		x.current[k], x.owner[k] = map[int]string{}, map[string]int{}
	}
	for _, r := range rows {
		x.owner[r.kind][r.slug] = r.id
		if r.current {
			x.current[r.kind][r.id] = r.slug
		}
	}
	return x
}

type Service struct {
	store   *database.Store
	index   atomic.Pointer[Index]
	Changed func()
}

func New(store *database.Store) *Service {
	s := &Service{store: store, Changed: func() {}}
	rows, err := s.load()
	if err != nil {
		store.Logf("slugs: %v", err)
	}
	s.index.Store(indexOf(rows))
	return s
}

func (s *Service) Index() *Index { return s.index.Load() }

func (s *Service) Refresh(snap *feeds.Snapshot, locate feeds.Locate) error {
	rows, err := s.load()
	if err != nil {
		return err
	}
	rows, moved := followStations(snap, rows)
	changed := append(assign(Station, stationItems(snap), rows, locate, s.store.Logf), assign(Line, lineItems(snap), rows, nil, s.store.Logf)...)
	if len(changed) == 0 && len(moved) == 0 {
		return nil
	}
	if err := s.save(moved, changed); err != nil {
		return err
	}
	if rows, err = s.load(); err != nil {
		return err
	}
	s.index.Store(indexOf(rows))
	s.store.Logf("slugs: %d addresses given, %d moved to a new station id", len(changed), len(moved))
	s.Changed()
	return nil
}

func (s *Service) load() ([]row, error) {
	return database.QueryRows(s.store.DB, `SELECT kind, slug, id, base, current FROM slugs ORDER BY kind, slug`, nil, func(rows *sql.Rows) (row, error) {
		var r row
		err := rows.Scan(&r.kind, &r.slug, &r.id, &r.base, &r.current)
		return r, err
	})
}

func (s *Service) save(moved, changed []row) error {
	tx, err := s.store.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range moved {
		if _, err := tx.Exec(`UPDATE slugs SET id = ? WHERE kind = ? AND slug = ?`, r.id, r.kind, r.slug); err != nil {
			return err
		}
	}
	for _, r := range changed {
		if _, err := tx.Exec(`UPDATE slugs SET current = 0 WHERE kind = ? AND id = ?`, r.kind, r.id); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO slugs (kind, slug, id, base, current) VALUES (?, ?, ?, ?, 1)`, r.kind, r.slug, r.id, r.base); err != nil {
			return err
		}
	}
	return tx.Commit()
}
