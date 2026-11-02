package transit

import (
	"cmp"
	"maps"
	"slices"
	"sync/atomic"
)

// Swapped whole, so readers never lock.
type keySet[K cmp.Ordered] struct{ keys atomic.Pointer[map[K]bool] }

func (k *keySet[K]) store(keys []K) {
	set := make(map[K]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	k.keys.Store(&set)
}

func (k *keySet[K]) all() map[K]bool {
	if set := k.keys.Load(); set != nil {
		return *set
	}
	return nil
}

func (k *keySet[K]) has(key K) bool { return k.all()[key] }

func (k *keySet[K]) sorted() []K { return slices.Sorted(maps.Keys(k.all())) }
