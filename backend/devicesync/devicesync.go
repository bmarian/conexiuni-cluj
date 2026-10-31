// Package devicesync keeps a person's devices in step (protocol.md): pairing slots in memory, chains of sealed records in their own database.
package devicesync

import (
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/web"
	"crypto/rand"
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Product limits; protocol.md §8 lists them for the clients.
const (
	pairTTL             = 5 * time.Minute
	maxOpenSlots        = 1000
	clientWindow        = 10 * time.Minute
	slotsPerClient      = 20
	missesPerClient     = 20
	clientsPerNetwork   = 10
	missesPerMinute     = 300
	chainsPerClient     = 5
	authMissesPerClient = 20
	writesPerDevice     = 120
	maxDevices          = 10
	unusedFor           = 365 * 24 * time.Hour
	pruneEvery          = time.Hour
)

type Options struct {
	// Cities is how many cities the server has: each may have a city, a news and a recents record, plus settings, options and devices.
	Cities int
	// MaxBytes stops new chains once the database is this large.
	MaxBytes int64
}

type Service struct {
	db         *sql.DB
	maxRecords int
	maxBytes   int64
	runner     *jobs.Runner

	mu           sync.Mutex
	slots        map[string]*slot
	slotsMade    *tiered
	misses       *tiered
	globalMisses *budget
	chainsMade   *budget
	authMisses   *budget
	writes       *budget
}

func Open(path string, opts Options) (*Service, error) {
	db, err := openStore(path)
	if err != nil {
		return nil, err
	}
	return &Service{
		db:           db,
		maxRecords:   3*opts.Cities + 3,
		maxBytes:     opts.MaxBytes,
		runner:       jobs.New(),
		slots:        map[string]*slot{},
		slotsMade:    newTiered(slotsPerClient, clientWindow),
		misses:       newTiered(missesPerClient, clientWindow),
		globalMisses: newBudget(missesPerMinute, time.Minute),
		chainsMade:   newBudget(chainsPerClient, 24*time.Hour),
		authMisses:   newBudget(authMissesPerClient, clientWindow),
		writes:       newBudget(writesPerDevice, time.Hour),
	}, nil
}

func (s *Service) Register(api fiber.Router) {
	g := api.Group("/sync", guard)
	g.Post("/pair", s.openSlot)
	g.Post("/pair/:code/claim", s.claim)
	g.Get("/pair/:code", s.readSlot)
	g.Put("/pair/:code/pub", s.reveal)
	g.Post("/pair/:code/messages", s.message)
	g.Delete("/pair/:code", s.closeSlot)
	g.Post("/chains", s.createChain)
	g.Post("/chains/:chain/devices", s.join)
	g.Delete("/chains/:chain/devices/self", s.leave)
	g.Get("/chains/:chain", s.state)
	g.Delete("/chains/:chain", s.deleteChain)
	g.Get("/chains/:chain/records/:name", s.readRecord)
	g.Put("/chains/:chain/records/:name", s.writeRecord)
	g.Get("/chains/:chain/wrap", s.readWrap)
	g.Post("/chains/:chain/rotate", s.rotate)
}

func guard(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !web.SameOrigin(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	return c.Next()
}

// Start deletes unused chains now and every pruneEvery after.
func (s *Service) Start() {
	s.runner.Go(func() {
		for {
			now := time.Now()
			if err := s.pruneChains(now); err != nil {
				log.Printf("sync: pruning unused chains: %v", err)
			}
			s.mu.Lock()
			s.pruneMemory(now)
			s.mu.Unlock()
			if !s.runner.Sleep(pruneEvery) {
				return
			}
		}
	})
}

func (s *Service) Close() error {
	s.runner.Stop()
	return s.db.Close()
}

// Callers hold s.mu.
func (s *Service) pruneMemory(now time.Time) {
	for code, sl := range s.slots {
		if now.After(sl.expires) {
			delete(s.slots, code)
		}
	}
	for _, b := range []interface{ prune(time.Time) }{s.slotsMade, s.misses, s.globalMisses, s.chainsMade, s.authMisses, s.writes} {
		b.prune(now)
	}
}

func randomBytes(n int) []byte {
	out := make([]byte, n)
	_, _ = rand.Read(out)
	return out
}

func randomID() string { return b64.EncodeToString(randomBytes(idSize)) }

// LogPath hides chain ids and record names, which would tie one person's devices together across addresses.
func LogPath(path string) string {
	rest, ok := strings.CutPrefix(path, "/api/sync/chains/")
	if !ok || rest == "" {
		return path
	}
	parts := strings.Split(rest, "/")
	parts[0] = "-"
	if len(parts) > 2 && parts[1] == "records" {
		parts[2] = "-"
	}
	return "/api/sync/chains/" + strings.Join(parts, "/")
}
