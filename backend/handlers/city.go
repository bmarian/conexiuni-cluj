package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/sources"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Always revalidate, so fixes reach clients on the next request.
const revalidateCacheControl = "no-cache"

type Options struct {
	VehicleInterval VehicleIntervalConfig
	LearningEnabled bool
	// Daily cap on vehicle samples taken for learning while nobody watches.
	LearningMaxQuota int
	// False keeps whatever the database holds; for development, where every fetch costs quota.
	Refresh bool
}

// City is one tenant: its sources, its database and everything learned from them.
type City struct {
	Slug     string
	Name     string
	Operator string
	Location *time.Location
	*database.Store

	Static     sources.Static
	Timetables sources.Timetables
	// nil for a schedule-only city.
	Realtime sources.Realtime
	// Vehicles for learning while nobody watches; Realtime when nil.
	Learning sources.Realtime
	Options  Options

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
	city := &City{Slug: slug, Name: name, Location: loc, Store: store, Static: static, Timetables: timetables, Realtime: realtime, Learning: learning, Options: opts}
	city.cityState = newCityState()
	city.hub = newLiveHub(city)
	return city
}

func (city *City) log(format string, args ...any) {
	log.Printf(city.Slug+": "+format, args...)
}

// Start runs the ingest and every learner; it returns at once.
func (city *City) Start() {
	city.StartCalendar()
	city.StartVacuumScheduler()
	city.StartStatsFlusher()
	city.StartVehicleLearningSampler()
	city.StartScheduleDelayLearner(city.Location)
	city.StartArchiver(city.Location)
	city.StartLive()
	city.startPushSenders()
	city.StartIngest()
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
	api.Get("/push/key", GetPushKey)
	api.Post("/push/subscribe", city.SubscribePush)
	api.Post("/push/unsubscribe", city.UnsubscribePush)
	api.Post("/push/reminders", city.SyncReminders)
}

// Pages whose HTML carries the route or stop in its meta tags.
func (city *City) RegisterPages(app fiber.Router) {
	app.Get("/", city.HomeOGHandler)
	app.Get("/route/:routeId/:direction", city.RouteOGHandler)
	app.Get("/stop/:stopId", city.StopOGHandler)
	app.Get("/plan", city.PlanOGHandler)
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
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Operator string `json:"operator"`
	// False for a schedule-only city: no buses on the map, arrivals from the timetable alone.
	Realtime bool `json:"realtime"`
	// The middle of the stops, where the map and the weather start.
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// The median stop: outlying villages on long lines do not pull it away from the centre.
func (city *City) info() cityInfo {
	info := cityInfo{Slug: city.Slug, Name: city.Name, Operator: city.Operator, Realtime: city.Realtime != nil}
	stops, err := city.GetStops(StopFilter{})
	if err != nil || len(stops) == 0 {
		return info
	}
	lats := make([]float64, len(stops))
	lons := make([]float64, len(stops))
	for i, s := range stops {
		lats[i], lons[i] = s.StopLat, s.StopLon
	}
	sort.Float64s(lats)
	sort.Float64s(lons)
	info.Lat, info.Lon = lats[len(lats)/2], lons[len(lons)/2]
	return info
}

// CitiesHandler lists the cities served, in display order.
func CitiesHandler(cities []*City) fiber.Handler {
	return func(c fiber.Ctx) error {
		out := make([]cityInfo, 0, len(cities))
		for _, city := range cities {
			out = append(out, city.info())
		}
		c.Set("Cache-Control", revalidateCacheControl)
		return c.JSON(out)
	}
}
