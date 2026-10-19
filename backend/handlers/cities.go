package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
)

var (
	ErrUnknownCity = errors.New("unknown city")
	ErrLastCity    = errors.New("the last city on cannot be turned off")
)

// Every city the server has; a disabled one is never shown, fetched or learned from.
type Cities struct {
	all  []*City
	path string
	mu   sync.Mutex
	// Bumped on every switch, so caches built from the list know to rebuild.
	version atomic.Int64
}

// The disabled slugs are kept as a JSON list at path.
func NewCities(all []*City, path string) (*Cities, error) {
	disabled, err := readDisabledCities(path)
	if err != nil {
		return nil, err
	}
	for _, city := range all {
		city.run.enabled.Store(!slices.Contains(disabled, city.Slug))
		if !city.Enabled() {
			city.log("disabled")
		}
	}
	cs := &Cities{all: all, path: path}
	if len(cs.Enabled()) == 0 {
		return nil, fmt.Errorf("%s turns off every city", path)
	}
	return cs, nil
}

func readDisabledCities(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var slugs []string
	if err := json.Unmarshal(data, &slugs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return slugs, nil
}

func (cs *Cities) All() []*City { return cs.all }

func (cs *Cities) Enabled() []*City {
	out := make([]*City, 0, len(cs.all))
	for _, city := range cs.all {
		if city.Enabled() {
			out = append(out, city)
		}
	}
	return out
}

// Nil for a disabled or unknown slug.
func (cs *Cities) Find(slug string) *City {
	for _, city := range cs.all {
		if city.Slug == slug && city.Enabled() {
			return city
		}
	}
	return nil
}

func (cs *Cities) Start() {
	for _, city := range cs.all {
		city.Start()
	}
}

func (cs *Cities) SetEnabled(slug string, on bool) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	i := slices.IndexFunc(cs.all, func(c *City) bool { return c.Slug == slug })
	if i < 0 {
		return ErrUnknownCity
	}
	city := cs.all[i]
	if city.Enabled() == on {
		return nil
	}
	if !on && len(cs.Enabled()) == 1 {
		return ErrLastCity
	}
	city.run.enabled.Store(on)
	if err := cs.save(); err != nil {
		city.run.enabled.Store(!on)
		return err
	}
	cs.version.Add(1)
	if on {
		city.log("enabled")
		go city.Start()
	} else {
		city.log("disabled, stopping")
		go city.Stop()
	}
	return nil
}

func (cs *Cities) save() error {
	disabled := []string{}
	for _, city := range cs.all {
		if !city.Enabled() {
			disabled = append(disabled, city.Slug)
		}
	}
	sort.Strings(disabled)
	data, err := json.Marshal(disabled)
	if err != nil {
		return err
	}
	tmp := cs.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, cs.path)
}
