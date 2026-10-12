package handlers

import (
	"bufio"
	"compress/gzip"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

// One poll loop feeds every /live subscriber its slice of the prediction table.

type liveSubscriber struct {
	stops map[int]struct{}
	trips map[string]struct{}
	ch    chan *liveTable
}

type liveHub struct {
	mu          sync.Mutex
	subscribers map[int64]*liveSubscriber
	nextID      int64
	running     bool
	tranzy      *tranzy.Client
	intervalCfg VehicleIntervalConfig
}

var VehicleHub *liveHub

func InitVehicleHub(tranzyClient *tranzy.Client, intervalCfg VehicleIntervalConfig) {
	VehicleHub = &liveHub{subscribers: make(map[int64]*liveSubscriber), tranzy: tranzyClient, intervalCfg: intervalCfg}
}

func (h *liveHub) intervalFor(subscribers int) time.Duration {
	return ComputeVehicleInterval(h.intervalCfg, subscribers, h.tranzy.VehiclesQuotaRemaining(), time.Now(), h.tranzy.Location())
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

func (h *liveHub) Subscribe(stopIDs []int, tripIDs []string) (*liveSubscriber, int64) {
	sub := &liveSubscriber{stops: make(map[int]struct{}, len(stopIDs)), trips: make(map[string]struct{}, len(tripIDs)), ch: make(chan *liveTable, 2)}
	for _, id := range stopIDs {
		sub.stops[id] = struct{}{}
	}
	for _, id := range tripIDs {
		sub.trips[NormalizeTripID(id)] = struct{}{}
	}
	h.mu.Lock()
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
	return sub, id
}

func (h *liveHub) Unsubscribe(id int64) {
	h.mu.Lock()
	_, ok := h.subscribers[id]
	delete(h.subscribers, id)
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
		if table, err := h.poll(interval); err != nil {
			log.Printf("live hub: poll failed interval=%s subs=%d err=%v", interval, n, err)
		} else {
			for _, sub := range subs {
				sub.offer(table)
			}
		}
		time.Sleep(interval)
	}
}

// poll fetches the vehicles (served from the cache within shelfLife) and returns the
// prediction table that fetch rebuilt.
func (h *liveHub) poll(shelfLife time.Duration) (*liveTable, error) {
	if _, err := GetVehicles(h.tranzy, shelfLife, VehicleFilter{}); err != nil {
		return nil, err
	}
	if table := currentLiveTable(); table != nil {
		return table, nil
	}
	return nil, fmt.Errorf("no prediction table yet")
}

// current is the table as of the latest vehicles, fetching them when the last are stale.
func (h *liveHub) current() (*liveTable, error) {
	if table := currentLiveTable(); table != nil && time.Since(table.AsOf) < 2*time.Minute {
		return table, nil
	}
	return h.poll(h.CurrentInterval())
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

// ServeLive answers /live: the prediction rows for the given stops and trips, and the
// vehicles on those trips, as an SSE stream of snapshot then deltas, or once as JSON.
func ServeLive(c fiber.Ctx) error {
	stopIDs, tripIDs := liveIDs(c)
	if len(stopIDs) == 0 && len(tripIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "stop_ids, trip_ids or route_ids required"})
	}
	if c.Query("once") != "" {
		sub := &liveSubscriber{stops: map[int]struct{}{}, trips: map[string]struct{}{}}
		for _, id := range stopIDs {
			sub.stops[id] = struct{}{}
		}
		for _, id := range tripIDs {
			sub.trips[NormalizeTripID(id)] = struct{}{}
		}
		table, err := VehicleHub.current()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(sub.view(table))
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
	sub, id := VehicleHub.Subscribe(stopIDs, tripIDs)
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer VehicleHub.Unsubscribe(id)
		out := &eventWriter{w: w}
		if gzipped {
			out.gz, _ = gzip.NewWriterLevel(w, gzip.BestSpeed)
		}
		defer out.close()
		keep := time.NewTicker(25 * time.Second)
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
