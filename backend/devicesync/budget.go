package devicesync

import (
	"conexiuni-cluj/web"
	"time"

	"github.com/gofiber/fiber/v3"
)

type tally struct {
	count int
	reset time.Time
}

// budget counts events per key in windows that start with each key's first event. Callers hold Service.mu.
type budget struct {
	limit  int
	window time.Duration
	counts map[string]*tally
}

func newBudget(limit int, window time.Duration) *budget {
	return &budget{limit: limit, window: window, counts: map[string]*tally{}}
}

func (b *budget) spent(key string, now time.Time) bool {
	counter := b.counts[key]
	return counter != nil && !now.After(counter.reset) && counter.count >= b.limit
}

func (b *budget) spend(key string, n int, now time.Time) {
	counter := b.counts[key]
	if counter == nil || now.After(counter.reset) {
		counter = &tally{reset: now.Add(b.window)}
		b.counts[key] = counter
	}
	counter.count += n
}

func (b *budget) prune(now time.Time) {
	for key, counter := range b.counts {
		if now.After(counter.reset) {
			delete(b.counts, key)
		}
	}
}

type requester struct{ client, network string }

func requesterOf(c fiber.Ctx) requester {
	return requester{web.ClientHash(c), web.NetworkKey(web.ClientIP(c))}
}

type tiered struct{ client, network *budget }

func newTiered(limit int, window time.Duration) *tiered {
	return &tiered{newBudget(limit, window), newBudget(limit*clientsPerNetwork, window)}
}

func (b *tiered) spent(who requester, now time.Time) bool {
	return b.client.spent(who.client, now) || b.network.spent(who.network, now)
}

func (b *tiered) spend(who requester, n int, now time.Time) {
	b.client.spend(who.client, n, now)
	b.network.spend(who.network, n, now)
}

func (b *tiered) prune(now time.Time) {
	b.client.prune(now)
	b.network.prune(now)
}
