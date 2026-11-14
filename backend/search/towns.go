package search

import (
	"log"
	"os"
	"sync"
	"sync/atomic"

	"conexiuni-cluj/planner"
	"conexiuni-cluj/router"
)

var (
	towns     atomic.Pointer[[]router.Town]
	readTowns sync.Once
)

// Once, in the background.
func ReadTowns() {
	readTowns.Do(func() { go loadTowns() })
}

// Empty until read.
func CountryTowns() []router.Town {
	if t := towns.Load(); t != nil {
		return *t
	}
	return nil
}

func loadTowns() {
	f, err := os.Open(planner.OSMPath())
	if err != nil {
		log.Printf("search: towns: %v", err)
		return
	}
	defer f.Close()
	found, err := router.Towns(f)
	if err != nil {
		log.Printf("search: towns: %v", err)
		return
	}
	towns.Store(&found)
}
