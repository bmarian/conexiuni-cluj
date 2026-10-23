package cities

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/destinations"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/ingest"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/live"
	"conexiuni-cluj/network"
	"conexiuni-cluj/planner"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/updates"
	"conexiuni-cluj/web"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

type City struct {
	Slug        string
	Name        string
	Operator    string
	OperatorURL string
	Location    *time.Location
	*database.Store
	*transit.Data

	Static     feeds.Static
	Timetables feeds.Timetables
	Realtime   feeds.Realtime
	Learning   feeds.Realtime
	News       feeds.News

	Live         *live.Engine
	Updates      *updates.Service
	Planner      *planner.Service
	Network      *network.Service
	Destinations *destinations.Service
	Ingest       *ingest.Importer

	offered transit.Availability
	run     cityRun
	centre  struct {
		sync.Mutex
		lat, lon float64
		ok       bool
	}
}

func New(slug, name string, store *database.Store, static feeds.Static, timetables feeds.Timetables, realtime, learning feeds.Realtime, opts live.Options) *City {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		log.Printf("%s: could not load Europe/Bucharest, using UTC: %v", slug, err)
		loc = time.UTC
	}
	if learning == nil {
		learning = realtime
	}
	c := &City{Slug: slug, Name: name, Location: loc, Store: store, Data: transit.New(store), Static: static, Timetables: timetables, Realtime: realtime, Learning: learning}
	c.Live = live.New(slug, loc, store, c.Data, &c.offered, realtime, learning, opts)
	c.Live.Runner = c.running
	c.Updates = updates.New(store, c.Data, loc)
	c.Planner = planner.New(loc, store, c.Data, c.Live)
	c.Network = network.New(loc, store, c.Data, &c.offered, c.Live)
	c.Network.Runner = c.running
	c.Planner.Colorful = c.Network.Colorful
	c.Updates.Colorful = c.Network.Colorful
	c.Destinations = destinations.New(c.Network, c.Planner.Places)
	forget := func() {
		c.Network.Invalidate()
		c.Planner.Forget()
	}
	c.Live.ScheduleChanged = forget
	c.Ingest = ingest.New(store, c.Data, &c.offered, c.Updates)
	c.Ingest.Static, c.Ingest.Timetables, c.Ingest.Locate, c.Ingest.Refresh = static, timetables, planner.Locate, opts.Refresh
	c.Ingest.NetworkStored = c.Live.NetworkStored
	c.Ingest.OffsetsStored = c.Live.OffsetsStored
	c.Ingest.Imported = func(stored bool) {
		if stored {
			forget()
		}
		c.refreshCentre()
		c.Live.RefreshTrips()
		c.Network.Warm()
		c.Planner.Warm()
	}
	c.run.enabled.Store(true)
	return c
}

// Availability is what the network offers: lines with departures and the stops they serve.
func (c *City) Availability() *transit.Availability { return &c.offered }

func (c *City) start(r *jobs.Runner) {
	c.refreshCentre()
	c.Live.StartCalendar(r)
	r.Go(func() { c.RunVacuumScheduler(r.Context()) })
	r.Go(func() { c.RunStatsFlusher(r.Context()) })
	c.Live.StartVehicleLearningSampler(r)
	c.Live.StartScheduleDelayLearner(r, c.Location)
	c.Live.StartArchiver(r, c.Location)
	c.Live.StartLive(r)
	c.Ingest.Start(r)
	c.Updates.StartNews(r, c.News)
}

func (c *City) Register(api fiber.Router) {
	api.Post("/stats/event", c.statsEvent)
	api.Get("/shapes", c.shapesHandler)
	api.Get("/live", c.Live.ServeLive)
	c.Planner.Register(api)
	c.Network.Register(api)
	c.Destinations.Register(api)
	c.Updates.Register(api)
}

type statsEventRequest struct {
	Metric string `json:"metric"`
	Key    string `json:"key"`
}

func validStatsEvent(metric, key string) bool {
	switch metric {
	case "pwa_install":
		return key == "appinstalled"
	}
	return false
}

func (city *City) statsEvent(c fiber.Ctx) error {
	if !web.SameOrigin(c) {
		return c.SendStatus(fiber.StatusNoContent)
	}
	var req statsEventRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	req.Metric = strings.TrimSpace(req.Metric)
	req.Key = strings.TrimSpace(req.Key)
	if !validStatsEvent(req.Metric, req.Key) {
		return c.SendStatus(fiber.StatusNoContent)
	}
	city.RecordEvent(req.Metric, req.Key)
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func (city *City) shapesHandler(c fiber.Ctx) error {
	var ids []string
	for _, id := range strings.Split(c.Query("shape_ids"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "shape_ids required"})
	}
	// No screen shows more shapes than the busiest station has lines.
	if most := city.offered.BusiestStation(); most > 0 && len(ids) > most {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("at most %d shape_ids", most)})
	}
	data, err := city.Shapes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Cache-Control", web.Revalidate)
	if c.Query("format") == "polyline" {
		return c.JSON(geo.Polylines(data))
	}
	return c.JSON(data)
}

type cityInfo struct {
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Operator    string          `json:"operator"`
	OperatorURL string          `json:"operatorUrl"`
	Realtime    bool            `json:"realtime"`
	News        bool            `json:"news"`
	Lat         float64         `json:"lat"`
	Lon         float64         `json:"lon"`
	Bounds      *network.Bounds `json:"bounds,omitempty"`
	Credits     []feeds.Credit  `json:"credits"`
}

func (c *City) credits() []feeds.Credit {
	out := []feeds.Credit{}
	for _, source := range []any{c.Static, c.Timetables, c.Realtime, c.Learning} {
		if credited, ok := source.(feeds.Credited); ok && !slices.Contains(out, credited.Credit()) {
			out = append(out, credited.Credit())
		}
	}
	return out
}

// The median, so outlying villages on long lines do not pull the centre away.
func (c *City) refreshCentre() {
	stops, err := c.Stops(transit.StopFilter{})
	if err != nil || len(stops) == 0 {
		return
	}
	lats := make([]float64, len(stops))
	lons := make([]float64, len(stops))
	for i, s := range stops {
		lats[i], lons[i] = s.StopLat, s.StopLon
	}
	sort.Float64s(lats)
	sort.Float64s(lons)
	c.centre.Lock()
	c.centre.lat, c.centre.lon, c.centre.ok = lats[len(lats)/2], lons[len(lons)/2], true
	c.centre.Unlock()
}

func (c *City) info() (cityInfo, bool) {
	info := cityInfo{Slug: c.Slug, Name: c.Name, Operator: c.Operator, OperatorURL: c.OperatorURL, Realtime: c.Realtime != nil, News: c.News != nil, Credits: c.credits()}
	c.centre.Lock()
	ok := c.centre.ok
	info.Lat, info.Lon = c.centre.lat, c.centre.lon
	c.centre.Unlock()
	if !ok {
		return info, false
	}
	info.Bounds = c.Network.Bounds()
	return info, true
}

// Handler lists the cities that are on, live ones first, then timetables only, each by name, with where it lies and what it offers.
func Handler(cities *Registry) fiber.Handler {
	return func(c fiber.Ctx) error {
		out := make([]cityInfo, 0, len(cities.all))
		for _, city := range cities.Enabled() {
			if info, ok := city.info(); ok {
				out = append(out, info)
			}
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Realtime != out[j].Realtime {
				return out[i].Realtime
			}
			return out[i].Name < out[j].Name
		})
		c.Set("Cache-Control", web.Revalidate)
		return c.JSON(out)
	}
}
