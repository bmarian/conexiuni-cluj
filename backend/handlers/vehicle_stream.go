package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/services/tranzy"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type vehicleHub struct {
	mu          sync.Mutex
	subscribers map[int64]*vehicleSubscriber
	nextID      int64
	running     bool
	tranzy      *tranzy.Client
	intervalCfg VehicleIntervalConfig
}

type vehicleSubscriber struct {
	tripIDs map[string]struct{}
	// Buffered so slow SSE writers don't block the hub.
	ch chan []models.Vehicle
}

var VehicleHub *vehicleHub

func InitVehicleHub(tranzyClient *tranzy.Client, intervalCfg VehicleIntervalConfig) {
	VehicleHub = &vehicleHub{
		subscribers: make(map[int64]*vehicleSubscriber),
		tranzy:      tranzyClient,
		intervalCfg: intervalCfg,
	}
}

func (h *vehicleHub) intervalFor(subscribers int) time.Duration {
	return ComputeVehicleInterval(
		h.intervalCfg,
		subscribers,
		h.tranzy.VehiclesQuotaRemaining(),
		time.Now(),
		h.tranzy.Location(),
	)
}

func (h *vehicleHub) CurrentInterval() time.Duration {
	h.mu.Lock()
	n := len(h.subscribers)
	h.mu.Unlock()
	return h.intervalFor(n)
}

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

	go h.sendTo(sub)

	if shouldStart {
		log.Printf("vehicle hub: starting poll loop")
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
	for {
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
		n := len(h.subscribers)
		h.mu.Unlock()

		interval := h.intervalFor(n)
		localNow := time.Now().In(h.tranzy.Location())
		slotLabel := "none"
		targetLabel := "-"
		if sl, ok := h.intervalCfg.SlotAt(localNow); ok {
			slotLabel = sl.label()
			targetLabel = sl.target(n).String()
		}
		log.Printf("vehicle hub: poll interval=%s target=%s slot=%s subs=%d",
			interval, targetLabel, slotLabel, n)

		vehicles, err := GetVehicles(h.tranzy, interval, VehicleFilter{})
		if err != nil {
			log.Printf("vehicle hub: poll fetch failed interval=%s subs=%d err=%v", interval, n, err)
		} else {
			for _, sub := range subs {
				broadcast(sub, vehicles)
			}
		}

		time.Sleep(interval)
	}
}

func (h *vehicleHub) sendTo(sub *vehicleSubscriber) {
	interval := h.CurrentInterval()
	vehicles, err := GetVehicles(h.tranzy, interval, VehicleFilter{})
	if err != nil {
		log.Printf("vehicle hub: initial fetch failed interval=%s err=%v", interval, err)
		return
	}
	broadcast(sub, vehicles)
}

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

func (s *vehicleSubscriber) Ch() <-chan []models.Vehicle { return s.ch }
