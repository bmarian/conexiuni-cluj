package devicesync

import (
	"conexiuni-cluj/web"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"math/big"
	"time"

	"github.com/gofiber/fiber/v3"
)

const pairSecretHeader = "X-Pair-Secret"

type side int

const (
	shower side = iota
	enterer
)

type slot struct {
	commit  []byte
	scanned bool
	secrets [2]string
	pubs    [2][]byte
	inbox   [2][]string
	sent    [2]int
	expires time.Time
}

func validCode(code string) bool {
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

// Callers hold s.mu.
func (s *Service) live(code string, now time.Time) *slot {
	if !validCode(code) {
		return nil
	}
	sl := s.slots[code]
	if sl == nil || now.After(sl.expires) {
		return nil
	}
	return sl
}

// Callers hold s.mu.
func (s *Service) sideOf(c fiber.Ctx, now time.Time) (*slot, side, bool) {
	sl := s.live(c.Params("code"), now)
	if sl == nil {
		return nil, 0, false
	}
	given := []byte(c.Get(pairSecretHeader))
	for _, sd := range []side{shower, enterer} {
		if sl.secrets[sd] != "" && subtle.ConstantTimeCompare([]byte(sl.secrets[sd]), given) == 1 {
			return sl, sd, true
		}
	}
	return nil, 0, false
}

func (s *Service) openSlot(c fiber.Ctx) error {
	var req struct {
		Commit *string `json:"commit"`
	}
	if !decode(c, 128, &req) || req.Commit == nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	commit, ok := fromB64(*req.Commit, sha256.Size)
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	client := web.ClientHash(c)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneMemory(now)
	if s.slotsMade.spent(client, now) {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	if len(s.slots) >= maxOpenSlots {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	var code string
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if code = fmt.Sprintf("%06d", n.Int64()); s.slots[code] == nil {
			break
		}
	}
	s.slotsMade.spend(client, 1, now)
	secret := randomID()
	s.slots[code] = &slot{commit: commit, secrets: [2]string{secret, ""}, expires: now.Add(pairTTL)}
	return c.JSON(fiber.Map{"code": code, "secret": secret, "expiresIn": int(pairTTL.Seconds())})
}

func (s *Service) claim(c fiber.Ctx) error {
	var req struct {
		Pub     *string `json:"pub"`
		Scanned *bool   `json:"scanned"`
	}
	if !decode(c, 256, &req) || req.Pub == nil || req.Scanned == nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	pub, ok := p256(*req.Pub)
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	client := web.ClientHash(c)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	// Refused before looking the code up, so a refusal says nothing about whether it exists.
	if s.globalMisses.spent("", now) || s.misses.spent(client, now) {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	sl := s.live(c.Params("code"), now)
	if sl == nil || sl.pubs[enterer] != nil {
		s.globalMisses.spend("", 1, now)
		s.misses.spend(client, 1, now)
		return c.SendStatus(fiber.StatusNotFound)
	}
	secret := randomID()
	sl.secrets[enterer], sl.pubs[enterer], sl.scanned = secret, pub, *req.Scanned
	return c.JSON(fiber.Map{"commit": b64.EncodeToString(sl.commit), "secret": secret})
}

func (s *Service) readSlot(c fiber.Ctx) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sl, sd, ok := s.sideOf(c, time.Now())
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	var pub any
	if other := sl.pubs[1-sd]; other != nil {
		pub = b64.EncodeToString(other)
	}
	messages := append([]string{}, sl.inbox[sd]...)
	return c.JSON(fiber.Map{"pub": pub, "scanned": sl.scanned, "messages": messages})
}

func (s *Service) reveal(c fiber.Ctx) error {
	var req struct {
		Pub *string `json:"pub"`
	}
	if !decode(c, 256, &req) || req.Pub == nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	pub, ok := p256(*req.Pub)
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl, sd, ok := s.sideOf(c, time.Now())
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if sd != shower || sl.pubs[enterer] == nil || sl.pubs[shower] != nil {
		return c.SendStatus(fiber.StatusConflict)
	}
	if sum := sha256.Sum256(pub); subtle.ConstantTimeCompare(sum[:], sl.commit) != 1 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	sl.pubs[shower] = pub
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Service) message(c fiber.Ctx) error {
	var req struct {
		Message *string `json:"message"`
	}
	if !decode(c, 1024, &req) || req.Message == nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if _, ok := fromB64(*req.Message, pairMessage); !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl, sd, ok := s.sideOf(c, time.Now())
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if sl.pubs[shower] == nil || sl.pubs[enterer] == nil || sl.sent[sd] >= 2 {
		return c.SendStatus(fiber.StatusConflict)
	}
	sl.inbox[1-sd] = append(sl.inbox[1-sd], *req.Message)
	sl.sent[sd]++
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Service) closeSlot(c fiber.Ctx) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, _, ok := s.sideOf(c, time.Now()); !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	delete(s.slots, c.Params("code"))
	return c.SendStatus(fiber.StatusNoContent)
}
