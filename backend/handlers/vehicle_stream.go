package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// vehicleHub coordinates one upstream vehicle poll for every connected
// subscriber. Instead of each browser tab firing its own `/api/vehicles`
// request on a timer, they all subscribe here and a single goroutine hits
// Tranzy every `pollInterval`, broadcasting each subscriber's filtered slice.
//
// Key property: the poller goroutine only exists while at least one subscriber
// is connected. When the last tab closes (or the user hides the tab), the
// goroutine exits and Tranzy stops receiving traffic entirely.
type vehicleHub struct {
	mu           sync.Mutex
	subscribers  map[int64]*vehicleSubscriber
	nextID       int64
	running      bool
	tranzy       *tranzy.Client
	pollInterval time.Duration
}

type vehicleSubscriber struct {
	tripIDs map[string]struct{}
	// Buffer=4: room for a few batches if the SSE writer briefly stalls, but
	// not unbounded so slow clients can't grow memory forever. If full we
	// drop the batch and they get the next tick.
	ch chan []models.Vehicle
}

var VehicleHub *vehicleHub

// InitVehicleHub must be called once at startup, before any SSE handler runs.
func InitVehicleHub(tranzyClient *tranzy.Client, pollInterval time.Duration) {
	VehicleHub = &vehicleHub{
		subscribers:  make(map[int64]*vehicleSubscriber),
		tranzy:       tranzyClient,
		pollInterval: pollInterval,
	}
}

// Subscribe registers interest in live updates for the given trip IDs. The
// returned channel receives a fresh `[]Vehicle` (filtered to tripIDs) on
// every broadcast. Call Unsubscribe with the returned id when done.
//
// The first batch is delivered immediately — it comes from the existing
// `GetVehicles` cache, so an early-arriving subscriber doesn't wait a full
// poll interval for data.
func (h *vehicleHub) Subscribe(tripIDs []string) (*vehicleSubscriber, int64) {
	set := make(map[string]struct{}, len(tripIDs))
	for _, id := range tripIDs {
		set[id] = struct{}{}
	}
	sub := &vehicleSubscriber{
		tripIDs: set,
		ch:      make(chan []models.Vehicle, 4),
	}

	h.mu.Lock()
	id := atomic.AddInt64(&h.nextID, 1)
	h.subscribers[id] = sub
	shouldStart := !h.running
	if shouldStart {
		h.running = true
	}
	total := len(h.subscribers)
	h.mu.Unlock()

	log.Printf("vehicle hub: +subscriber %d (total=%d, trips=%d)", id, total, len(tripIDs))

	// Fire an immediate send. Reuses the upstream cache so it's free if
	// another tab already fetched recently.
	go h.sendTo(sub)

	if shouldStart {
		log.Printf("vehicle hub: starting poll loop (every %s)", h.pollInterval)
		go h.run()
	}
	return sub, id
}

func (h *vehicleHub) Unsubscribe(id int64) {
	h.mu.Lock()
	_, ok := h.subscribers[id]
	if ok {
		delete(h.subscribers, id)
	}
	remaining := len(h.subscribers)
	h.mu.Unlock()
	if ok {
		log.Printf("vehicle hub: -subscriber %d (remaining=%d)", id, remaining)
	}
}

func (h *vehicleHub) run() {
	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.Lock()
		if len(h.subscribers) == 0 {
			h.running = false
			h.mu.Unlock()
			log.Printf("vehicle hub: no subscribers, stopping poll loop")
			return
		}
		subs := make([]*vehicleSubscriber, 0, len(h.subscribers))
		for _, s := range h.subscribers {
			subs = append(subs, s)
		}
		h.mu.Unlock()

		// One upstream fetch per tick, via the existing cached path. TTL is
		// the poll interval so every tick actually refreshes.
		vehicles, err := GetVehicles(h.tranzy, h.pollInterval, VehicleFilter{})
		if err != nil {
			log.Printf("vehicle hub: GetVehicles: %v", err)
			continue
		}
		for _, sub := range subs {
			broadcast(sub, vehicles)
		}
	}
}

// sendTo delivers the currently-cached vehicle snapshot to one subscriber
// out-of-band. Used on join so new subscribers don't wait a full tick.
func (h *vehicleHub) sendTo(sub *vehicleSubscriber) {
	vehicles, err := GetVehicles(h.tranzy, h.pollInterval, VehicleFilter{})
	if err != nil {
		log.Printf("vehicle hub: initial GetVehicles: %v", err)
		return
	}
	broadcast(sub, vehicles)
}

// broadcast filters `vehicles` down to the subscriber's trip set and does a
// non-blocking send. If the channel is full (slow reader), the batch is
// dropped — the subscriber picks up fresh data on the next tick.
func broadcast(sub *vehicleSubscriber, vehicles []models.Vehicle) {
	filtered := make([]models.Vehicle, 0, len(vehicles))
	for _, v := range vehicles {
		if _, ok := sub.tripIDs[v.TripID]; ok {
			filtered = append(filtered, v)
		}
	}
	select {
	case sub.ch <- filtered:
	default:
	}
}

// Ch exposes the subscriber channel to the SSE handler.
func (s *vehicleSubscriber) Ch() <-chan []models.Vehicle { return s.ch }
