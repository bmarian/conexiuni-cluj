package cities

import (
	"conexiuni-cluj/config"
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/gtfs"
	"conexiuni-cluj/feeds/gtfsrt"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/feeds/traces"
	"conexiuni-cluj/feeds/tranzy"
	"conexiuni-cluj/live"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env is what a city's feeds are built from: the settings, the city's own database and, once it runs, its patterns.
type Env struct {
	Slug   string
	Config *config.Config
	open   func() (*database.Store, error)
	store  *database.Store
	err    error
	place  placement
	traces *traces.Recorder
}

func NewEnv(slug string, cfg *config.Config, open func() (*database.Store, error)) *Env {
	return &Env{Slug: slug, Config: cfg, open: open}
}

// Store opens the city's database on first use.
func (e *Env) Store() (*database.Store, error) {
	if e.store == nil && e.err == nil {
		e.store, e.err = e.open()
	}
	return e.store, e.err
}

// IDs maps a feed's own ids to the app's numbers; nil when the database would not open, which Store reports.
func (e *Env) IDs() feeds.IDMap {
	if store, err := e.Store(); err == nil {
		return store
	}
	return nil
}

// Traces records the buses of the lines the city's feeds lack routes for; nil when the database would not open, which Store reports.
func (e *Env) Traces() *traces.Recorder {
	if e.traces == nil {
		if store, err := e.Store(); err == nil {
			e.traces = traces.NewRecorder(store, live.StationaryRadiusMeters)
		}
	}
	return e.traces
}

// Placement answers once the city runs; until then no stop is on any pattern.
func (e *Env) Placement() feeds.Placement { return &e.place }

// Tracing answers once the city runs; until then there are no lines.
func (e *Env) Tracing() feeds.Tracing { return &e.place }

// What a running city tells its feeds.
type Located interface {
	feeds.Placement
	feeds.Tracing
}

func (e *Env) Place(city Located) { e.place.city = city }

type placement struct{ city Located }

func (p *placement) StopIndex(routeID, direction, stopID int) int {
	if p.city == nil {
		return -1
	}
	return p.city.StopIndex(routeID, direction, stopID)
}

func (p *placement) RouteIDs() map[string]int {
	if p.city == nil {
		return nil
	}
	return p.city.RouteIDs()
}

func (p *placement) Along(routeID, direction int, lat, lon float64) (float64, float64, bool) {
	if p.city == nil {
		return 0, 0, false
	}
	return p.city.Along(routeID, direction, lat, lon)
}

// GTFS is a whole network from a GTFS zip, its times included.
func (e *Env) GTFS(url string) feeds.Static {
	return gtfs.New(url, e.Config.TimetableRefresh, e.IDs())
}

// GTFSTimetables reads only the departures of a GTFS zip, for a network that comes from elsewhere.
func (e *Env) GTFSTimetables(url string) feeds.Timetables {
	return gtfs.New(url, e.Config.TimetableRefresh, nil)
}

// GTFSRo is the zip external.gtfs.ro keeps for a city.
func GTFSRo(city string) string {
	return "https://external.gtfs.ro/" + city + "/" + strings.ToUpper(city) + ".zip"
}

func (e *Env) GTFSRT(url string) feeds.Realtime {
	source := gtfsrt.New(url, e.IDs())
	source.Placement = e.Placement()
	return source
}

func (e *Env) News(url string, parse news.Parser) feeds.News {
	return news.New(url, parse, e.Config.NewsRefresh)
}

func (e *Env) RSS(url string) feeds.News { return e.News(url, news.RSS) }

// Tranzy fills Static, Realtime and, with a second key, Learning, from TRANZY_API_KEY_{SLUG} and TRANZY_API_KEY_{SLUG}_LEARNING.
func (e *Env) Tranzy(agencyID int) (Feeds, error) {
	name := "TRANZY_API_KEY_" + strings.ToUpper(e.Slug)
	if e.Config.Environment == "development" {
		name = "TRANZY_API_KEY_DEVELOPMENT"
	}
	key, learningKey := os.Getenv(name), os.Getenv(name+"_LEARNING")
	if key == "" {
		return Feeds{}, fmt.Errorf("%w: no %s", ErrNoKey, name)
	}
	store, err := e.Store()
	if err != nil {
		return Feeds{}, err
	}
	c, agency := e.Config, strconv.Itoa(agencyID)
	source := tranzy.New(tranzy.NewClient(c.TranzyBaseUrl, key, agency, c.TranzyRateLimit, c.TranzyVehiclesDailyQuota, c.TranzyDefaultDailyQuota,
		quotas{e.Slug, store, true}))
	out := Feeds{Static: source, Realtime: source}
	if learningKey != "" && learningKey != key {
		out.Learning = tranzy.New(tranzy.NewClientWithQuotaNames(c.TranzyBaseUrl, learningKey, agency, c.TranzyRateLimit, c.TranzyVehiclesDailyQuota, c.TranzyDefaultDailyQuota,
			tranzy.QuotaNames{Vehicles: "vehicle_learning_vehicles", Default: "vehicle_learning_default"}, quotas{e.Slug, store, false}))
	}
	return out, nil
}

// Keeps a Tranzy key's daily counts across restarts.
type quotas struct {
	slug  string
	store *database.Store
	stats bool
}

func (q quotas) Load(name string) (int, time.Time, error) {
	count, resetAt, err := q.store.LoadQuota(name)
	if err != nil {
		err = fmt.Errorf("%s: %w", q.slug, err)
	}
	return count, resetAt, err
}

func (q quotas) Save(name string, count int, resetAt time.Time) error {
	if err := q.store.SaveQuota(name, count, resetAt); err != nil {
		return fmt.Errorf("%s: %w", q.slug, err)
	}
	return nil
}

func (q quotas) RecordUsage(name string) {
	if q.stats {
		q.store.RecordQuotaUsage(name)
	}
}
