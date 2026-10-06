package handlers

import (
	"bytes"
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

// Moves settings and favorites between devices: the receiver shows a code, the
// sender posts its export to it. Links only live in memory, for a few minutes.
const (
	transferLinkTTL          = 10 * time.Minute
	maxTransferLinks         = 1000
	maxTransferBody          = 32 << 10
	maxTransferLinkMisses    = 20
	transferLinkSecretHeader = "X-Link-Secret"
)

type transferLink struct {
	secret  string
	expires time.Time
	payload []byte
}

type transferLinkMisses struct {
	count int
	reset time.Time
}

var (
	transferLinksMu    sync.Mutex
	transferLinks      = map[string]*transferLink{}
	transferLinkMissed = map[string]*transferLinkMisses{}
)

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

// Callers hold transferLinksMu.
func pruneTransferLinks(now time.Time) {
	for code, link := range transferLinks {
		if now.After(link.expires) {
			delete(transferLinks, code)
		}
	}
	for client, misses := range transferLinkMissed {
		if now.After(misses.reset) {
			delete(transferLinkMissed, client)
		}
	}
}

// Callers hold transferLinksMu.
func liveTransferLink(code string, now time.Time) *transferLink {
	if !validTransferCode(code) {
		return nil
	}
	link := transferLinks[code]
	if link == nil || now.After(link.expires) {
		return nil
	}
	return link
}

// Callers hold transferLinksMu.
func ownedTransferLink(c fiber.Ctx, now time.Time) *transferLink {
	link := liveTransferLink(c.Params("code"), now)
	if link == nil || subtle.ConstantTimeCompare([]byte(link.secret), []byte(c.Get(transferLinkSecretHeader))) != 1 {
		return nil
	}
	return link
}

func CreateTransferLink(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !sameOriginRequest(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	now := time.Now()

	transferLinksMu.Lock()
	defer transferLinksMu.Unlock()
	pruneTransferLinks(now)
	if len(transferLinks) >= maxTransferLinks {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	var code string
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		code = fmt.Sprintf("%06d", n.Int64())
		if transferLinks[code] == nil {
			break
		}
	}
	link := &transferLink{secret: hex.EncodeToString(secret), expires: now.Add(transferLinkTTL)}
	transferLinks[code] = link
	return c.JSON(fiber.Map{"code": code, "secret": link.secret, "expiresIn": int(transferLinkTTL.Seconds())})
}

// The body is the app's own settings export. Each receiver checks what it applies,
// so the server only makes sure it is a small JSON object.
func SendToTransferLink(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !sameOriginRequest(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	body := bytes.TrimSpace(c.Body())
	if len(body) > maxTransferBody {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	if !bytes.HasPrefix(body, []byte("{")) || !json.Valid(body) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	client := ClientHashFromLocals(c)
	now := time.Now()

	transferLinksMu.Lock()
	defer transferLinksMu.Unlock()
	// Six digits are guessable, so a client that keeps missing is turned away for a while.
	misses := transferLinkMissed[client]
	if misses != nil && now.After(misses.reset) {
		delete(transferLinkMissed, client)
		misses = nil
	}
	if misses != nil && misses.count >= maxTransferLinkMisses {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	link := liveTransferLink(c.Params("code"), now)
	if link == nil {
		if misses == nil {
			misses = &transferLinkMisses{reset: now.Add(transferLinkTTL)}
			transferLinkMissed[client] = misses
		}
		misses.count++
		return c.SendStatus(fiber.StatusNotFound)
	}
	link.payload = bytes.Clone(body)
	return c.SendStatus(fiber.StatusNoContent)
}

// The link stays until the receiver deletes it, so a reply lost while the watch
// sleeps can be fetched again.
func ReadTransferLink(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	transferLinksMu.Lock()
	defer transferLinksMu.Unlock()
	link := ownedTransferLink(c, time.Now())
	if link == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if link.payload == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(link.payload)
}

func DeleteTransferLink(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	transferLinksMu.Lock()
	defer transferLinksMu.Unlock()
	if ownedTransferLink(c, time.Now()) == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	delete(transferLinks, c.Params("code"))
	return c.SendStatus(fiber.StatusNoContent)
}
