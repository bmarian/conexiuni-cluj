package admin

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const appScope = "app"

type problem struct {
	Key       string `json:"key"`
	Text      string `json:"text"`
	Dismissed bool   `json:"dismissed"`
}

type dismissals struct {
	path string
	mu   sync.Mutex
	at   map[string]time.Time
}

func openDismissals(path string) *dismissals {
	d := &dismissals{path: path, at: map[string]time.Time{}}
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &d.at)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("admin: dismissed problems %s: %v", path, err)
		d.at = map[string]time.Time{}
	}
	return d
}

func dismissalID(scope, key string) string { return scope + "|" + key }

func (d *dismissals) set(scope, key string, dismissed bool, now time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := dismissalID(scope, key)
	if dismissed {
		d.at[id] = now
	} else {
		delete(d.at, id)
	}
	return d.saveLocked()
}

// A dismissal ends when its problem is gone.
func (d *dismissals) apply(scope string, problems []problem) []problem {
	d.mu.Lock()
	defer d.mu.Unlock()
	current := map[string]bool{}
	for i := range problems {
		id := dismissalID(scope, problems[i].Key)
		current[id] = true
		_, problems[i].Dismissed = d.at[id]
	}
	changed := false
	for id := range d.at {
		if strings.HasPrefix(id, scope+"|") && !current[id] {
			delete(d.at, id)
			changed = true
		}
	}
	if changed {
		_ = d.saveLocked()
	}
	return problems
}

func (d *dismissals) saveLocked() error {
	data, err := json.Marshal(d.at)
	if err != nil {
		return err
	}
	tmp := d.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, d.path)
}

func registerDismissals(admin fiber.Router, d *dismissals, session fiber.Handler) {
	admin.Put("/problems", session, func(c fiber.Ctx) error {
		var req struct {
			Scope     string `json:"scope"`
			Key       string `json:"key"`
			Dismissed *bool  `json:"dismissed"`
		}
		if err := c.Bind().Body(&req); err != nil || req.Scope == "" || req.Key == "" || req.Dismissed == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		if err := d.set(req.Scope, req.Key, *req.Dismissed, time.Now()); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"ok": true})
	})
}
