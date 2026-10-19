package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/sources"
	"fmt"
	"log"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Always revalidate, so fixes reach clients on the next request.
const revalidateCacheControl = "no-cache"

type Options struct {
	VehicleInterval  VehicleIntervalConfig
	LearningEnabled  bool
	LearningMaxQuota int
	// False reuses whatever the database holds.
	Refresh bool
}

type City struct {
	Slug        string
	Name        string
	Operator    string
	OperatorURL string
	Location    *time.Location
	*database.Store

	Static     sources.Static
	Timetables sources.Timetables
	// nil for a schedule-only city.
	Realtime sources.Realtime
	Learning sources.Realtime
	News     sources.News
	// Names the town and street of stops that share a name.
	Locate  sources.Locate
	Options Options

	run cityRun
	cityState
}

func NewCity(slug, name string, store *database.Store, static sources.Static, timetables sources.Timetables, realtime, learning sources.Realtime, opts Options) *City {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		log.Printf("%s: could not load Europe/Bucharest, using UTC: %v", slug, err)
		loc = time.UTC
	}
	if learning == nil {
		learning = realtime
	}
	city := &City{Slug: slug, Name: name, Location: loc, Store: store, Static: static, Timetables: timetables, Realtime: realtime, Learning: learning, Locate: osmLocate, Options: opts}
	city.cityState = newCityState()
	city.hub = newLiveHub(city)
	city.run.enabled.Store(true)
	return city
}

// StopIndex implements sources.Placement.
func (city *City) StopIndex(routeID, direction, stopID int) int {
	city.liveTrips.RLock()
	lt := city.liveTrips.byTrip[fmt.Sprintf("%d_%d", routeID, direction)]
	city.liveTrips.RUnlock()
	if lt == nil {
		return -1
	}
	for i, stop := range lt.trip.Stops {
		if stop.ID == stopID {
			return i
		}
	}
	return -1
}

func (city *City) log(format string, args ...any) {
	log.Printf(city.Slug+": "+format, args...)
}

func (city *City) start(r *runner) {
	city.refreshCentre()
	city.StartCalendar(r)
	r.Go(func() { city.RunVacuumScheduler(r.ctx) })
	r.Go(func() { city.RunStatsFlusher(r.ctx) })
	city.StartVehicleLearningSampler(r)
	city.StartScheduleDelayLearner(r, city.Location)
	city.StartArchiver(r, city.Location)
	city.StartLive(r)
	city.startPushSenders(r)
	city.StartIngest(r)
	city.StartNews(r)
}

func (city *City) Register(api fiber.Router) {
	api.Post("/stats/event", city.statsEvent)
	api.Get("/shapes", city.shapesHandler)
	api.Get("/live", city.ServeLive)
	api.Get("/plan", city.handlePlan)
	api.Get("/places", city.handlePlaces)
	api.Get("/network", city.NetworkHandler())
	api.Get("/network/version", city.NetworkVersionHandler())
	api.Get("/route-changes", city.GetRouteChanges)
	api.Get("/updates", city.GetUpdates)
	api.Get("/push/key", GetPushKey)
	api.Post("/push/subscribe", city.SubscribePush)
	api.Post("/push/unsubscribe", city.UnsubscribePush)
	api.Post("/push/reminders", city.SyncReminders)
}

func (city *City) RegisterPages(app fiber.Router) {
	app.Get("/", city.HomeOGHandler)
	app.Get("/route/:routeId/:direction", city.RouteOGHandler)
	app.Get("/stop/:stopId", city.StopOGHandler)
	app.Get("/plan", city.PlanOGHandler)
}

func RegisterLegacyPages(app fiber.Router, slug string) {
	for _, path := range []string{"/stop/*", "/route/*", "/plan", "/settings", "/nav"} {
		app.Get(path, func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusMovedPermanently).To("/" + slug + c.OriginalURL())
		})
	}
}

func sameOriginRequest(c fiber.Ctx) bool {
	switch c.Get("Sec-Fetch-Site") {
	case "same-origin", "same-site", "none":
		return true
	}
	origin := c.Get(fiber.HeaderOrigin)
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, c.Hostname())
}

type statsEventRequest struct {
	Metric string `json:"metric"`
	Key    string `json:"key"`
}

func validStatsEvent(metric, key string) bool {
	switch metric {
	case "pwa_install":
		return key == "appinstalled"
	case "host_move":
		return key == "sent" || key == "empty" || key == "received"
	}
	return false
}

func (city *City) statsEvent(c fiber.Ctx) error {
	if !sameOriginRequest(c) {
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
	data, err := city.GetShapes(ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Cache-Control", revalidateCacheControl)
	if c.Query("format") == "polyline" {
		return c.JSON(shapePolylines(data))
	}
	return c.JSON(data)
}

type cityInfo struct {
	Slug        string           `json:"slug"`
	Name        string           `json:"name"`
	Operator    string           `json:"operator"`
	OperatorURL string           `json:"operatorUrl"`
	Realtime    bool             `json:"realtime"`
	News        bool             `json:"news"`
	Lat         float64          `json:"lat"`
	Lon         float64          `json:"lon"`
	Bounds      *Bounds          `json:"bounds,omitempty"`
	Credits     []sources.Credit `json:"credits"`
}

type Bounds struct {
	South float64 `json:"south"`
	West  float64 `json:"west"`
	North float64 `json:"north"`
	East  float64 `json:"east"`
}

func (city *City) credits() []sources.Credit {
	out := []sources.Credit{}
	for _, source := range []any{city.Static, city.Timetables, city.Realtime, city.Learning} {
		if credited, ok := source.(sources.Credited); ok && !slices.Contains(out, credited.Credit()) {
			out = append(out, credited.Credit())
		}
	}
	return out
}

// The median, so outlying villages on long lines do not pull the centre away.
func (city *City) refreshCentre() {
	stops, err := city.GetStops(StopFilter{})
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
	city.centre.Lock()
	city.centre.lat, city.centre.lon, city.centre.ok = lats[len(lats)/2], lons[len(lons)/2], true
	city.centre.Unlock()
}

func (city *City) info() (cityInfo, bool) {
	info := cityInfo{Slug: city.Slug, Name: city.Name, Operator: city.Operator, OperatorURL: city.OperatorURL, Realtime: city.Realtime != nil, News: city.News != nil, Credits: city.credits()}
	city.centre.Lock()
	ok := city.centre.ok
	info.Lat, info.Lon = city.centre.lat, city.centre.lon
	city.centre.Unlock()
	if !ok {
		return info, false
	}
	city.networkCache.Lock()
	if built := city.networkCache.current; built != nil {
		info.Bounds = built.bounds
	}
	city.networkCache.Unlock()
	return info, true
}

func CitiesHandler(cities *Cities) fiber.Handler {
	return func(c fiber.Ctx) error {
		out := make([]cityInfo, 0, len(cities.all))
		for _, city := range cities.Enabled() {
			if info, ok := city.info(); ok {
				out = append(out, info)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		c.Set("Cache-Control", revalidateCacheControl)
		return c.JSON(out)
	}
}
