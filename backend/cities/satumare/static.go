package satumare

import (
	"conexiuni-cluj/feeds"
	"time"
)

// static is the zip's network with tusm.ro's departures over its weekday-only ones, read in one go so no refresh stores the zip's times alone.
type static struct {
	zip   feeds.Static
	times timetables
}

func (s *static) Cadence() time.Duration { return s.zip.Cadence() }

func (s *static) Snapshot() (*feeds.Snapshot, error) {
	snap, err := s.zip.Snapshot()
	if err != nil {
		return nil, err
	}
	if snap.Timetables, err = s.times.Timetables(snap); err != nil {
		return nil, err
	}
	return snap, nil
}
