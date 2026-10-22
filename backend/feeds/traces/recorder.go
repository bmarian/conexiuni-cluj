// Package traces records where the buses of chosen lines go and learns those lines' routes from it, for networks whose feeds lack them.
package traces

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/predictions"
	"slices"
	"sync"
	"time"
)

type Store interface {
	TracedLines() ([]string, error)
	SetTracedLines(lines []string) error
	AddLineFixes(fixes []database.LineFix) error
	LineFixes(line string, since time.Time) ([]database.LineFix, error)
	PruneLineFixes(line string, before time.Time) error
}

// Recorder keeps one row per place a traced bus moved to, from its first to its last sighting there, so stands show; one per city, shared by its live reader and its network source.
type Recorder struct {
	store Store
	// A bus nearer than this to where it stood has not moved.
	stationary float64

	mu     sync.Mutex
	lines  map[string]bool
	loaded bool
	open   map[string]database.LineFix
}

func NewRecorder(store Store, stationaryMeters float64) *Recorder {
	return &Recorder{store: store, stationary: stationaryMeters, open: map[string]database.LineFix{}}
}

// Sighting is a bus of a traced line as one poll of a live feed saw it.
type Sighting struct {
	Line, Vehicle string
	Lat, Lon      float64
}

// Traced tells whether a line's buses are recorded.
func (r *Recorder) Traced(line string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.loaded {
		lines, err := r.store.TracedLines()
		if err != nil {
			return false
		}
		r.setLines(lines)
	}
	return r.lines[line]
}

func (r *Recorder) setLines(lines []string) {
	r.lines, r.loaded = map[string]bool{}, true
	for _, l := range lines {
		r.lines[l] = true
	}
}

// Record takes every traced bus one poll saw; a bus missing from it has left where it stood.
func (r *Recorder) Record(seen []Sighting, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var done []database.LineFix
	here := make(map[string]bool, len(seen))
	for _, s := range seen {
		here[s.Vehicle] = true
		f, ok := r.open[s.Vehicle]
		if ok && f.Line == s.Line && predictions.HaversineMeters(f.Lat, f.Lon, s.Lat, s.Lon) <= r.stationary {
			f.Last = now
			r.open[s.Vehicle] = f
			continue
		}
		if ok {
			done = append(done, f)
		}
		r.open[s.Vehicle] = database.LineFix{Line: s.Line, Vehicle: s.Vehicle, Lat: s.Lat, Lon: s.Lon, First: now, Last: now}
	}
	for vehicle, f := range r.open {
		if !here[vehicle] {
			done = append(done, f)
			delete(r.open, vehicle)
		}
	}
	return r.store.AddLineFixes(done)
}

// Trace makes these the traced lines, each recorded from its time on (when its route last changed, or as far back as is kept): older fixes are dropped and the rest returned.
func (r *Recorder) Trace(since map[string]time.Time) (map[string]*Line, error) {
	names := make([]string, 0, len(since))
	for name := range since {
		names = append(names, name)
	}
	slices.Sort(names)
	if err := r.store.SetTracedLines(names); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.setLines(names)
	r.mu.Unlock()
	out := make(map[string]*Line, len(names))
	for _, name := range names {
		if err := r.store.PruneLineFixes(name, since[name]); err != nil {
			return nil, err
		}
		fixes, err := r.store.LineFixes(name, since[name])
		if err != nil {
			return nil, err
		}
		out[name] = newLine(fixes)
	}
	return out, nil
}
