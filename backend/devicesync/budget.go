package devicesync

import "time"

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
