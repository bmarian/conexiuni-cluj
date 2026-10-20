package cities

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
type Registry struct {
	all  []*City
	path string
	mu   sync.Mutex
	// Bumped on every switch, so caches built from the list know to rebuild.
	version atomic.Int64
}

// The disabled slugs are kept as a JSON list at path.
func NewRegistry(all []*City, path string) (*Registry, error) {
	disabled, err := readDisabledCities(path)
	if err != nil {
		return nil, err
	}
	for _, c := range all {
		c.run.enabled.Store(!slices.Contains(disabled, c.Slug))
		if !c.Enabled() {
			c.Logf("disabled")
		}
	}
	cs := &Registry{all: all, path: path}
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

func (cs *Registry) All() []*City { return cs.all }

// Version changes whenever a city is switched on or off.
func (cs *Registry) Version() int64 { return cs.version.Load() }

func (cs *Registry) Enabled() []*City {
	out := make([]*City, 0, len(cs.all))
	for _, c := range cs.all {
		if c.Enabled() {
			out = append(out, c)
		}
	}
	return out
}

// Nil for a disabled or unknown slug.
func (cs *Registry) Find(slug string) *City {
	for _, c := range cs.all {
		if c.Slug == slug && c.Enabled() {
			return c
		}
	}
	return nil
}

func (cs *Registry) Start() {
	for _, c := range cs.all {
		c.Start()
	}
}

// Stops every city's background work and waits for it.
func (cs *Registry) Shutdown() {
	for _, c := range cs.all {
		c.run.enabled.Store(false)
		c.Stop()
	}
}

func (cs *Registry) SetEnabled(slug string, on bool) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	i := slices.IndexFunc(cs.all, func(c *City) bool { return c.Slug == slug })
	if i < 0 {
		return ErrUnknownCity
	}
	c := cs.all[i]
	if c.Enabled() == on {
		return nil
	}
	if !on && len(cs.Enabled()) == 1 {
		return ErrLastCity
	}
	c.run.enabled.Store(on)
	if err := cs.save(); err != nil {
		c.run.enabled.Store(!on)
		return err
	}
	cs.version.Add(1)
	if on {
		c.Logf("enabled")
		go c.Start()
	} else {
		c.Logf("disabled, stopping")
		go c.Stop()
	}
	return nil
}

func (cs *Registry) save() error {
	disabled := []string{}
	for _, c := range cs.all {
		if !c.Enabled() {
			disabled = append(disabled, c.Slug)
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
