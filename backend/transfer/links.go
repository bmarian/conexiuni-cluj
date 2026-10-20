// Package transfer moves a person's settings to another device: the receiver shows a code and the sender posts its export to it. Kept in memory only.
package transfer

import (
	"bytes"
	"conexiuni-cluj/web"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	transferLinkTTL          = 10 * time.Minute
	maxTransferLinks         = 1000
	maxTransferBody          = 32 << 10
	transferLinkSecretHeader = "X-Link-Secret"

	// Six digits are guessable: misses are budgeted per client and globally, about 0.3% per code.
	maxTransferMissesPerClient  = 20
	maxTransferMissesPerMinute  = 300
	maxTransferCreatesPerClient = 20
)

type slot struct {
	secret  string
	expires time.Time
	payload []byte
}

type tally struct {
	count int
	reset time.Time
}

type Links struct {
	mu           sync.Mutex
	links        map[string]*slot
	clientMisses map[string]*tally
	creates      map[string]*tally
	misses       tally
}

func New() *Links {
	return &Links{links: map[string]*slot{}, clientMisses: map[string]*tally{}, creates: map[string]*tally{}}
}

func (l *Links) Register(api fiber.Router) {
	api.Post("/transfer", l.create)
	api.Put("/transfer/:code", l.send)
	api.Get("/transfer/:code", l.read)
	api.Delete("/transfer/:code", l.delete)
}

func validTransferCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Callers hold l.mu.
func (l *Links) prune(now time.Time) {
	for code, link := range l.links {
		if now.After(link.expires) {
			delete(l.links, code)
		}
	}
	for _, counters := range []map[string]*tally{l.clientMisses, l.creates} {
		for client, counter := range counters {
			if now.After(counter.reset) {
				delete(counters, client)
			}
		}
	}
}

// Callers hold l.mu.
func overLimit(counters map[string]*tally, client string, limit int, now time.Time) bool {
	counter := counters[client]
	return counter != nil && !now.After(counter.reset) && counter.count >= limit
}

// Callers hold l.mu.
func bump(counters map[string]*tally, client string, now time.Time) {
	counter := counters[client]
	if counter == nil || now.After(counter.reset) {
		counter = &tally{reset: now.Add(transferLinkTTL)}
		counters[client] = counter
	}
	counter.count++
}

// Callers hold l.mu.
func (l *Links) live(code string, now time.Time) *slot {
	if !validTransferCode(code) {
		return nil
	}
	link := l.links[code]
	if link == nil || now.After(link.expires) {
		return nil
	}
	return link
}

// Callers hold l.mu.
func (l *Links) owned(c fiber.Ctx, now time.Time) *slot {
	link := l.live(c.Params("code"), now)
	if link == nil || subtle.ConstantTimeCompare([]byte(link.secret), []byte(c.Get(transferLinkSecretHeader))) != 1 {
		return nil
	}
	return link
}

func (l *Links) create(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !web.SameOrigin(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	client := web.ClientHash(c)
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	if overLimit(l.creates, client, maxTransferCreatesPerClient, now) {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	if len(l.links) >= maxTransferLinks {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	var code string
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		code = fmt.Sprintf("%06d", n.Int64())
		if l.links[code] == nil {
			break
		}
	}
	bump(l.creates, client, now)
	link := &slot{secret: hex.EncodeToString(secret), expires: now.Add(transferLinkTTL)}
	l.links[code] = link
	return c.JSON(fiber.Map{"code": code, "secret": link.secret, "expiresIn": int(transferLinkTTL.Seconds())})
}

// Receivers check what they apply; the server only checks it is a small JSON object.
func (l *Links) send(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !web.SameOrigin(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	body := bytes.TrimSpace(c.Body())
	if len(body) > maxTransferBody {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	if !bytes.HasPrefix(body, []byte("{")) || !json.Valid(body) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	client := web.ClientHash(c)
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	if now.After(l.misses.reset) {
		l.misses = tally{reset: now.Add(time.Minute)}
		// Also bounds the per-client counters when nobody is creating codes.
		l.prune(now)
	}
	// Refused before looking the code up, so a refusal says nothing about whether it exists.
	if l.misses.count >= maxTransferMissesPerMinute ||
		overLimit(l.clientMisses, client, maxTransferMissesPerClient, now) {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	link := l.live(c.Params("code"), now)
	if link == nil {
		l.misses.count++
		bump(l.clientMisses, client, now)
		return c.SendStatus(fiber.StatusNotFound)
	}
	link.payload = bytes.Clone(body)
	return c.SendStatus(fiber.StatusNoContent)
}

// Kept until the receiver deletes it, so a reply lost while the watch sleeps can be fetched again.
func (l *Links) read(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	l.mu.Lock()
	defer l.mu.Unlock()
	link := l.owned(c, time.Now())
	if link == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if link.payload == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(link.payload)
}

func (l *Links) delete(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.owned(c, time.Now()) == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	delete(l.links, c.Params("code"))
	return c.SendStatus(fiber.StatusNoContent)
}
