package handlers

import (
	"bufio"
	"compress/gzip"
	"conexiuni-cluj/database"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

// One poll loop feeds every /live subscriber its slice of the prediction table.

const (
	// A household or an office behind NAT.
	maxLiveStreamsPerClient = 32
	// Under the 60 s read timeout of nginx and the apps.
	liveKeepAlive = 25 * time.Second
)

type liveSubscriber struct {
	client string
	person bool
	stops  map[int]struct{}
	trips  map[string]struct{}
	ch     chan *liveTable
}

type liveHub struct {
	mu          sync.Mutex
	subscribers map[int64]*liveSubscriber
	perClient   map[string]int
	nextID      int64
	running     bool
	city        *City
}

func newLiveHub(city *City) *liveHub {
	return &liveHub{subscribers: make(map[int64]*liveSubscriber), perClient: make(map[string]int), city: city}
}

func (h *liveHub) intervalFor(subscribers int) time.Duration {
	// A schedule-only city has nothing to spend; the table still rolls forward at the viewers' pace.
	remaining := math.MaxInt32
	if h.city.Realtime != nil {
		if budget := h.city.Realtime.Budget(); budget.Limit > 0 {
			remaining = budget.Remaining
		}
	}
	return ComputeVehicleInterval(h.city.Options.VehicleInterval, subscribers, remaining, time.Now(), h.city.Location)
}

func (h *liveHub) CurrentInterval() time.Duration {
	h.mu.Lock()
	n := len(h.subscribers)
	h.mu.Unlock()
	return h.intervalFor(n)
}

func (h *liveHub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers)
}

type liveWatching struct {
	Streams int
	People  int
	Stops   map[int]int
	Routes  map[int]int
}

// Watching counts the people on each stop and line among the open streams.
func (h *liveHub) Watching() liveWatching {
	city := h.city
	stops := make(map[int]map[string]struct{})
	trips := make(map[string]map[string]struct{})
	people := make(map[string]struct{})
	w := liveWatching{}
	h.mu.Lock()
	for _, sub := range h.subscribers {
		if !sub.person {
			continue
		}
		w.Streams++
		people[sub.client] = struct{}{}
		for id := range sub.stops {
			addWatcher(stops, id, sub.client)
		}
		for trip := range sub.trips {
			addWatcher(trips, trip, sub.client)
		}
	}
	h.mu.Unlock()
	w.People = len(people)

	w.Stops = make(map[int]int, len(stops))
	for id, clients := range stops {
		w.Stops[id] = len(clients)
	}
	routes := make(map[int]map[string]struct{})
	for trip, clients := range trips {
		if routeID, ok := city.routeIDOfTrip(trip); ok {
			for client := range clients {
				addWatcher(routes, routeID, client)
			}
		}
	}
	w.Routes = make(map[int]int, len(routes))
	for id, clients := range routes {
		w.Routes[id] = len(clients)
	}
	return w
}

func addWatcher[K comparable](m map[K]map[string]struct{}, id K, client string) {
	if m[id] == nil {
		m[id] = make(map[string]struct{})
	}
	m[id][client] = struct{}{}
}

func (city *City) routeIDOfTrip(tripID string) (int, bool) {
	city.liveTrips.RLock()
	lt, ok := city.liveTrips.byTrip[tripID]
	city.liveTrips.RUnlock()
	if !ok {
		return 0, false
	}
	return lt.routeID, true
}

// Each person counts once per stop and line a day, however often their screens reconnect.

func (city *City) recordLiveViews(client string, stopIDs []int, tripIDs []string) {
	if client == "" {
		return
	}
	day := database.StatsDate(time.Now())
	city.liveViews.Lock()
	defer city.liveViews.Unlock()
	if city.liveViews.day != day {
		city.liveViews.day, city.liveViews.seen = day, make(map[string]struct{})
	}
	note := func(metric string, id int) {
		key := strconv.Itoa(id)
		if _, ok := city.liveViews.seen[client+metric+key]; ok {
			return
		}
		city.liveViews.seen[client+metric+key] = struct{}{}
		city.RecordEvent(metric, key)
	}
	for _, id := range stopIDs {
		if !city.availability.IsReady() || city.availability.StopHasBuses(id) {
			note("stop_view", id)
		}
	}
	for _, trip := range tripIDs {
		if routeID, ok := city.routeIDOfTrip(trip); ok {
			note("route_view", routeID)
		}
	}
}

func (h *liveHub) Subscribe(client string, person bool, stopIDs []int, tripIDs []string) (*liveSubscriber, int64, bool) {
	sub := &liveSubscriber{client: client, person: person, stops: make(map[int]struct{}, len(stopIDs)), trips: make(map[string]struct{}, len(tripIDs)), ch: make(chan *liveTable, 2)}
	for _, id := range stopIDs {
		sub.stops[id] = struct{}{}
	}
	for _, id := range tripIDs {
		sub.trips[id] = struct{}{}
	}
	h.mu.Lock()
	if h.perClient[client] >= maxLiveStreamsPerClient {
		h.mu.Unlock()
		return nil, 0, false
	}
	h.perClient[client]++
	id := atomic.AddInt64(&h.nextID, 1)
	h.subscribers[id] = sub
	start := !h.running
	if start {
		h.running = true
	}
	total := len(h.subscribers)
	h.mu.Unlock()
	log.Printf("live hub: +subscriber %d (total=%d, stops=%d, trips=%d)", id, total, len(stopIDs), len(tripIDs))
	go h.sendTo(sub)
	if start {
		go h.run()
	}
	return sub, id, true
}

func (h *liveHub) Unsubscribe(id int64) {
	h.mu.Lock()
	sub, ok := h.subscribers[id]
	if ok {
		delete(h.subscribers, id)
		if h.perClient[sub.client]--; h.perClient[sub.client] <= 0 {
			delete(h.perClient, sub.client)
		}
	}
	remaining := len(h.subscribers)
	h.mu.Unlock()
	if ok {
		log.Printf("live hub: -subscriber %d (remaining=%d)", id, remaining)
	}
}

func (h *liveHub) run() {
	for {
		h.mu.Lock()
		if len(h.subscribers) == 0 {
			h.running = false
			h.mu.Unlock()
			return
		}
		subs := make([]*liveSubscriber, 0, len(h.subscribers))
		for _, s := range h.subscribers {
			subs = append(subs, s)
		}
		n := len(h.subscribers)
		h.mu.Unlock()

		interval := h.intervalFor(n)
		if table, err := h.poll(); err != nil {
			log.Printf("live hub: poll failed interval=%s subs=%d err=%v", interval, n, err)
		} else {
			for _, sub := range subs {
				sub.offer(table)
			}
		}
		time.Sleep(interval)
	}
}

func (h *liveHub) poll() (*liveTable, error) {
	city := h.city
	if city.Realtime == nil {
		return city.RecomputeLive(nil, time.Now()), nil
	}
	if _, err := city.observeVehicles(city.Realtime); err != nil {
		return nil, err
	}
	return city.currentLiveTable(), nil
}

func (h *liveHub) current() (*liveTable, error) {
	city := h.city
	if table := city.currentLiveTable(); table != nil && time.Since(table.AsOf) < 2*time.Minute {
		return table, nil
	}
	return h.poll()
}

func (h *liveHub) sendTo(sub *liveSubscriber) {
	table, err := h.current()
	if err != nil {
		log.Printf("live hub: initial fetch failed: %v", err)
		return
	}
	sub.offer(table)
}

func (s *liveSubscriber) offer(table *liveTable) {
	select {
	case s.ch <- table:
	default:
	}
}

type liveView struct {
	AsOf     int64         `json:"as_of"`
	Rows     []liveRow     `json:"rows"`
	Vehicles []liveVehicle `json:"vehicles"`
}

type liveDelta struct {
	AsOf     int64     `json:"as_of"`
	Rows     []liveRow `json:"rows"`
	Vehicles struct {
		Upsert []liveVehicle `json:"upsert"`
		Remove []int         `json:"remove"`
	} `json:"vehicles"`
}

func (s *liveSubscriber) view(table *liveTable) liveView {
	out := liveView{AsOf: table.AsOf.Unix(), Rows: []liveRow{}, Vehicles: []liveVehicle{}}
	for stopID := range s.stops {
		out.Rows = append(out.Rows, table.rowsByStop[stopID]...)
	}
	for tripID := range s.trips {
		for _, row := range table.rowsByTrip[tripID] {
			if _, dup := s.stops[row.StopID]; !dup {
				out.Rows = append(out.Rows, row)
			}
		}
		out.Vehicles = append(out.Vehicles, table.vehiclesByTrip[tripID]...)
	}
	return out
}

func rowKey(r liveRow) string { return r.TripID + "@" + strconv.Itoa(r.StopID) }

func sameEntries(a, b []liveEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func diffViews(prev, next liveView) (liveDelta, bool) {
	delta := liveDelta{AsOf: next.AsOf, Rows: []liveRow{}}
	delta.Vehicles.Upsert, delta.Vehicles.Remove = []liveVehicle{}, []int{}
	before := make(map[string]liveRow, len(prev.Rows))
	for _, r := range prev.Rows {
		before[rowKey(r)] = r
	}
	for _, r := range next.Rows {
		if old, ok := before[rowKey(r)]; !ok || !sameEntries(old.Entries, r.Entries) {
			delta.Rows = append(delta.Rows, r)
		}
	}
	seen := make(map[int]liveVehicle, len(prev.Vehicles))
	for _, v := range prev.Vehicles {
		seen[v.ID] = v
	}
	now := make(map[int]struct{}, len(next.Vehicles))
	for _, v := range next.Vehicles {
		now[v.ID] = struct{}{}
		if old, ok := seen[v.ID]; !ok || old != v {
			delta.Vehicles.Upsert = append(delta.Vehicles.Upsert, v)
		}
	}
	for id := range seen {
		if _, still := now[id]; !still {
			delta.Vehicles.Remove = append(delta.Vehicles.Remove, id)
		}
	}
	changed := len(delta.Rows) > 0 || len(delta.Vehicles.Upsert) > 0 || len(delta.Vehicles.Remove) > 0
	return delta, changed
}

func liveIDs(c fiber.Ctx) ([]int, []string) {
	var stops []int
	for _, s := range strings.Split(c.Query("stop_ids"), ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			stops = append(stops, id)
		}
	}
	var trips []string
	for _, s := range strings.Split(c.Query("trip_ids"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			trips = append(trips, s)
		}
	}
	for _, s := range strings.Split(c.Query("route_ids"), ",") {
		if _, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			s = strings.TrimSpace(s)
			trips = append(trips, s+OUTGOING_SUFFIX, s+INCOMING_SUFFIX)
		}
	}
	return stops, trips
}

// SSE stream of a snapshot then deltas, or one JSON answer.
func (city *City) ServeLive(c fiber.Ctx) error {
	stopIDs, tripIDs := liveIDs(c)
	if len(stopIDs) == 0 && len(tripIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "stop_ids, trip_ids or route_ids required"})
	}
	client, person := ClientHashFromLocals(c), IsPerson(c)
	if person {
		city.recordLiveViews(client, stopIDs, tripIDs)
	}
	if c.Query("once") != "" {
		sub := &liveSubscriber{stops: map[int]struct{}{}, trips: map[string]struct{}{}}
		for _, id := range stopIDs {
			sub.stops[id] = struct{}{}
		}
		for _, id := range tripIDs {
			sub.trips[id] = struct{}{}
		}
		table, err := city.hub.current()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(sub.view(table))
	}

	sub, id, ok := city.hub.Subscribe(client, person, stopIDs, tripIDs)
	if !ok {
		// A 200 that ends at once makes a browser reconnect after `retry`; a 429 would stop it for good.
		log.Printf("live hub: refused a stream for client %s, over %d", client, maxLiveStreamsPerClient)
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-store")
		return c.SendString(fmt.Sprintf("retry: %d\n\n", liveKeepAlive.Milliseconds()))
	}
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Set("Vary", "Accept-Encoding")
	gzipped := strings.Contains(c.Get("Accept-Encoding"), "gzip")
	if gzipped {
		c.Set("Content-Encoding", "gzip")
	}
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer city.hub.Unsubscribe(id)
		out := &eventWriter{w: w}
		if gzipped {
			out.gz, _ = gzip.NewWriterLevel(w, gzip.BestSpeed)
		}
		defer out.close()
		keep := time.NewTicker(liveKeepAlive)
		defer keep.Stop()
		started := false
		var last liveView
		for {
			select {
			case table, ok := <-sub.ch:
				if !ok {
					return
				}
				next := sub.view(table)
				var event string
				var payload any
				if !started {
					started, event, payload = true, "snapshot", next
				} else {
					delta, changed := diffViews(last, next)
					if !changed {
						continue
					}
					event, payload = "delta", delta
				}
				last = next
				data, err := json.Marshal(payload)
				if err != nil {
					continue
				}
				if err := out.write(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)); err != nil {
					return
				}
			case <-keep.C:
				if err := out.write(": ping\n\n"); err != nil {
					return
				}
			}
		}
	})
}

type eventWriter struct {
	w  *bufio.Writer
	gz *gzip.Writer
}

func (e *eventWriter) write(text string) error {
	if e.gz != nil {
		if _, err := e.gz.Write([]byte(text)); err != nil {
			return err
		}
		if err := e.gz.Flush(); err != nil {
			return err
		}
	} else if _, err := e.w.WriteString(text); err != nil {
		return err
	}
	return e.w.Flush()
}

func (e *eventWriter) close() {
	if e.gz != nil {
		_ = e.gz.Close()
		_ = e.w.Flush()
	}
}
