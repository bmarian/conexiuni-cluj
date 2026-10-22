package push

import (
	"bytes"
	"conexiuni-cluj/database"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/updates"
	"conexiuni-cluj/web"
	"crypto/ecdh"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/gofiber/fiber/v3"
)

const (
	pushTTLSeconds      = 3 * 24 * 60 * 60
	pushRecordSize      = 2048
	maxPushEndpointLen  = 1024
	maxPushRouteNameLen = 32

	pushWindowStartHour = 8
	pushWindowEndHour   = 20

	// A household or an office behind NAT.
	maxPushSignupsPerClient = 16
	pushSignupWindow        = 24 * time.Hour
)

// The server POSTs to whatever endpoint it is given, so only browser push services are accepted.
var pushServiceHosts = []string{
	"fcm.googleapis.com",
	"push.services.mozilla.com",
	"notify.windows.com",
	"push.apple.com",
}

type pushConfig struct {
	publicKey  string
	privateKey string
	subject    string
}

// Calls one client made since reset.
type clientCounter struct {
	count int
	reset time.Time
}

var (
	pushCfg    *pushConfig
	pushClient = &http.Client{Timeout: 15 * time.Second}

	pushSignupsMu sync.Mutex
	pushSignups   = map[string]*clientCounter{}
)

type pushText struct {
	line, timetable, stops, and                      string
	days                                             map[string]string
	timetableOne, timetableMany, timetableAny        string
	stopAdded, stopsAdded, stopRemoved, stopsRemoved string
	stopsBoth                                        string
}

// Mirrors the routeChange* strings in frontend/src/locales.
var pushTexts = map[string]pushText{
	"ro": {
		line:      "Linia %s · %s",
		timetable: "Orar modificat",
		stops:     "Traseu modificat",
		and:       "și",
		days: map[string]string{
			"weekdays": "din timpul săptămânii",
			"saturday": "de sâmbătă",
			"sunday":   "de duminică",
		},
		timetableOne:  "Orarul %s s-a schimbat.",
		timetableMany: "Orarele %s s-au schimbat.",
		timetableAny:  "Orarul s-a schimbat.",
		stopAdded:     "A fost adăugată o stație.",
		stopsAdded:    "Au fost adăugate stații.",
		stopRemoved:   "A fost scoasă o stație.",
		stopsRemoved:  "Au fost scoase stații.",
		stopsBoth:     "Au fost adăugate și scoase stații.",
	},
	"en": {
		line:      "Line %s · %s",
		timetable: "Timetable changed",
		stops:     "Route changed",
		and:       "and",
		days: map[string]string{
			"weekdays": "weekday",
			"saturday": "Saturday",
			"sunday":   "Sunday",
		},
		timetableOne:  "The %s timetable changed.",
		timetableMany: "The %s timetables changed.",
		timetableAny:  "The timetable changed.",
		stopAdded:     "A stop was added.",
		stopsAdded:    "Stops were added.",
		stopRemoved:   "A stop was removed.",
		stopsRemoved:  "Stops were removed.",
		stopsBoth:     "Stops were added and removed.",
	},
}

type pushMessage struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Tag       string `json:"tag"`
	Timestamp int64  `json:"timestamp"`
	// Stays up until it's dealt with.
	Alert bool `json:"alert,omitempty"`
}

type pushSubscriber struct {
	endpoint, p256dh, auth, locale string
}

type pushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	Routes []string `json:"routes"`
	Locale string   `json:"locale"`
}

// Configure turns push on for every city with the server's VAPID keys.
func Configure(publicKey, privateKey, subject string) error {
	publicKey = strings.TrimRight(strings.TrimSpace(publicKey), "=")
	privateKey = strings.TrimRight(strings.TrimSpace(privateKey), "=")
	if publicKey == "" || privateKey == "" {
		return errors.New("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY are not set")
	}
	if err := checkVAPIDKeys(publicKey, privateKey); err != nil {
		return err
	}
	// webpush-go adds "mailto:" itself unless the subject is an https URL.
	subject = strings.TrimPrefix(strings.TrimSpace(subject), "mailto:")
	if subject == "" {
		subject = web.SiteOrigin
	}

	pushCfg = &pushConfig{publicKey: publicKey, privateKey: privateKey, subject: subject}
	log.Printf("push: notifications enabled, sent between %02d:00 and %02d:00", pushWindowStartHour, pushWindowEndHour)
	return nil
}

func (s *Service) Start(r *jobs.Runner) {
	if pushCfg == nil {
		return
	}
	if s.wake == nil {
		s.wake = make(chan struct{}, 1)
	}
	r.Go(func() { s.runPushSender(r, s.loc) })
	r.Go(func() { s.runReminderSender(r, s.loc) })
}

// Changes found overnight (the 04:00 warmup, a deploy) wait for the morning instead of waking people up.
func (s *Service) runPushSender(r *jobs.Runner, loc *time.Location) {
	for {
		now := time.Now().In(loc)
		if !inPushWindow(now) {
			if !r.Sleep(time.Until(nextPushWindow(now))) {
				return
			}
			continue
		}
		s.flushPendingPushes()
		select {
		case <-s.wake:
		case <-r.Done():
			return
		}
	}
}

func inPushWindow(t time.Time) bool {
	return t.Hour() >= pushWindowStartHour && t.Hour() < pushWindowEndHour
}

func nextPushWindow(now time.Time) time.Time {
	start := time.Date(now.Year(), now.Month(), now.Day(), pushWindowStartHour, 0, 0, 0, now.Location())
	if !start.After(now) {
		start = start.AddDate(0, 0, 1)
	}
	return start
}

func checkVAPIDKeys(publicKey, privateKey string) error {
	pub, err := base64.RawURLEncoding.DecodeString(publicKey)
	if err != nil {
		return fmt.Errorf("VAPID_PUBLIC_KEY: %w", err)
	}
	priv, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil {
		return fmt.Errorf("VAPID_PRIVATE_KEY: %w", err)
	}
	key, err := ecdh.P256().NewPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("VAPID_PRIVATE_KEY: %w", err)
	}
	if !bytes.Equal(key.PublicKey().Bytes(), pub) {
		return errors.New("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY are not from the same key pair")
	}
	return nil
}

func key(c fiber.Ctx) error {
	if pushCfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "push notifications are not configured"})
	}
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(fiber.Map{"public_key": pushCfg.publicKey})
}

func (s *Service) subscribe(c fiber.Ctx) error {
	if pushCfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "push notifications are not configured"})
	}
	if !web.SameOrigin(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	var req pushSubscriptionRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if !validPushEndpoint(req.Endpoint) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported push endpoint"})
	}
	if !validPushKey(req.Keys.P256dh, 65) || !validPushKey(req.Keys.Auth, 16) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid subscription keys"})
	}
	if ok, err := s.admitPushEndpoint(web.ClientHash(c), req.Endpoint, time.Now()); err != nil {
		s.store.Logf("push: could not look up subscription: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not save subscription"})
	} else if !ok {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	locale := "ro"
	if req.Locale == "en" {
		locale = "en"
	}
	sub := pushSubscriber{endpoint: req.Endpoint, p256dh: req.Keys.P256dh, auth: req.Keys.Auth, locale: locale}
	if err := s.savePushSubscription(sub, cleanPushRouteNames(req.Routes)); err != nil {
		s.store.Logf("push: could not save subscription: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not save subscription"})
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Service) unsubscribe(c fiber.Ctx) error {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := c.Bind().Body(&req); err != nil || req.Endpoint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := s.deletePushSubscription(req.Endpoint); err != nil {
		s.store.Logf("push: could not delete subscription: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not delete subscription"})
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func validPushEndpoint(endpoint string) bool {
	if len(endpoint) > maxPushEndpointLen {
		return false
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range pushServiceHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// Re-saving a known endpoint is free; every new one spends the address's sign-ups.
func (s *Service) admitPushEndpoint(client, endpoint string, now time.Time) (bool, error) {
	var known int
	err := s.store.DB.QueryRow(`SELECT 1 FROM push_subscriptions WHERE endpoint = ?`, endpoint).Scan(&known)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	pushSignupsMu.Lock()
	defer pushSignupsMu.Unlock()
	for c, counter := range pushSignups {
		if now.After(counter.reset) {
			delete(pushSignups, c)
		}
	}
	counter := pushSignups[client]
	if counter == nil {
		counter = &clientCounter{reset: now.Add(pushSignupWindow)}
		pushSignups[client] = counter
	}
	if counter.count >= maxPushSignupsPerClient {
		return false, nil
	}
	counter.count++
	return true, nil
}

func validPushKey(key string, size int) bool {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(key, "="))
	if err != nil || len(raw) != size {
		return false
	}
	return size != 65 || raw[0] == 0x04
}

func cleanPushRouteNames(names []string) []string {
	seen := make(map[string]bool)
	out := []string{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > maxPushRouteNameLen || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) == updates.MaxFilterNames {
			break
		}
	}
	return out
}

func (s *Service) savePushSubscription(sub pushSubscriber, routes []string) error {
	tx, err := s.store.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO push_subscriptions (endpoint, p256dh, auth, locale, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
			p256dh = excluded.p256dh, auth = excluded.auth, locale = excluded.locale, updated_at = excluded.updated_at`,
		sub.endpoint, sub.p256dh, sub.auth, sub.locale, time.Now().UnixMilli()); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM push_subscription_routes WHERE endpoint = ?`, sub.endpoint); err != nil {
		return err
	}
	for _, name := range routes {
		if _, err := tx.Exec(`INSERT INTO push_subscription_routes (endpoint, route_short_name) VALUES (?, ?)`, sub.endpoint, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) deletePushSubscription(endpoint string) error {
	tx, err := s.store.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM push_subscription_routes WHERE endpoint = ?`, endpoint); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM push_reminders WHERE endpoint = ?`, endpoint); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Wake() {
	if s.wake == nil {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) flushPendingPushes() {
	changes, err := database.QueryRows(s.store.DB, `
		SELECT id, route_short_name, source, detected_at, changes
		FROM route_changes
		WHERE push_pending = 1
		ORDER BY id DESC`,
		nil,
		func(rows *sql.Rows) (updates.RouteChange, error) {
			var rc updates.RouteChange
			var payload string
			if err := rows.Scan(&rc.ID, &rc.RouteShortName, &rc.Source, &rc.DetectedAt, &payload); err != nil {
				return rc, err
			}
			return rc, json.Unmarshal([]byte(payload), &rc.Changes)
		})
	if err != nil {
		s.store.Logf("push: could not load pending changes: %v", err)
		return
	}
	if len(changes) == 0 {
		return
	}

	// Notifications for a line share a tag, so an older one would only be replaced by the newest.
	notified := make(map[string]bool)
	for _, change := range changes {
		if !notified[change.RouteShortName] {
			notified[change.RouteShortName] = true
			s.sendRouteChangePush(change)
		}
	}
	if _, err := s.store.DB.Exec(`UPDATE route_changes SET push_pending = 0 WHERE push_pending = 1 AND id <= ?`,
		changes[0].ID); err != nil {
		s.store.Logf("push: could not mark changes as sent: %v", err)
	}
}

func (s *Service) sendRouteChangePush(change updates.RouteChange) {
	subscribers, err := database.QueryRows(s.store.DB, `
		SELECT s.endpoint, s.p256dh, s.auth, s.locale
		FROM push_subscriptions s
		JOIN push_subscription_routes r ON r.endpoint = s.endpoint
		WHERE r.route_short_name = ?`,
		[]any{change.RouteShortName},
		func(rows *sql.Rows) (pushSubscriber, error) {
			var s pushSubscriber
			return s, rows.Scan(&s.endpoint, &s.p256dh, &s.auth, &s.locale)
		})
	if err != nil {
		s.store.Logf("push: could not load followers of line %s: %v", change.RouteShortName, err)
		return
	}
	if len(subscribers) == 0 {
		return
	}
	if change.RouteID == 0 {
		_ = s.store.DB.QueryRow(`SELECT COALESCE(MAX(route_id), 0) FROM routes WHERE route_short_name = ?`,
			change.RouteShortName).Scan(&change.RouteID)
	}

	delivered := 0
	for _, sub := range subscribers {
		payload, err := json.Marshal(routeChangePushMessage(s.slug, change, sub.locale))
		if err != nil {
			s.store.Logf("push: could not encode line %s change: %v", change.RouteShortName, err)
			return
		}
		if s.sendPush(sub, payload) {
			delivered++
		}
	}
	s.store.Logf("push: line %s change %d sent to %d/%d followers", change.RouteShortName, change.ID, delivered, len(subscribers))
}

func (s *Service) sendPush(sub pushSubscriber, payload []byte) bool {
	return s.sendPushWith(sub, payload, pushTTLSeconds, webpush.UrgencyNormal)
}

func (s *Service) sendPushWith(sub pushSubscriber, payload []byte, ttl int, urgency webpush.Urgency) bool {
	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.endpoint,
		Keys:     webpush.Keys{P256dh: sub.p256dh, Auth: sub.auth},
	}, &webpush.Options{
		HTTPClient:      pushClient,
		RecordSize:      pushRecordSize,
		Subscriber:      pushCfg.subject,
		TTL:             ttl,
		Urgency:         urgency,
		VAPIDPublicKey:  pushCfg.publicKey,
		VAPIDPrivateKey: pushCfg.privateKey,
	})
	host := pushEndpointHost(sub.endpoint)
	if err != nil {
		s.store.Logf("push: sending to %s failed: %v", host, err)
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		if err := s.deletePushSubscription(sub.endpoint); err != nil {
			s.store.Logf("push: could not drop expired subscription on %s: %v", host, err)
		}
	default:
		s.store.Logf("push: %s rejected a notification with status %d: %s", host, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return false
}

// Endpoint paths are per-browser secrets, so only the host is logged.
func pushEndpointHost(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil {
		return u.Host
	}
	return "?"
}

func routeChangePushMessage(slug string, change updates.RouteChange, locale string) pushMessage {
	text, ok := pushTexts[locale]
	if !ok {
		text = pushTexts["ro"]
	}
	what := text.timetable
	if change.Source == updates.SourceStops {
		what = text.stops
	}
	target := "/" + slug + "/"
	if change.RouteID != 0 {
		direction := "0"
		if len(change.Changes) > 0 && change.Changes[0].Direction != "" {
			direction = change.Changes[0].Direction
		}
		target = fmt.Sprintf("/%s/route/%d/%s?change=%d", slug, change.RouteID, direction, change.ID)
	}
	return pushMessage{
		Title:     fmt.Sprintf(text.line, change.RouteShortName, what),
		Body:      routeChangeSummary(change, text),
		URL:       target,
		Tag:       "route-change-" + change.RouteShortName,
		Timestamp: change.DetectedAt,
	}
}

func routeChangeSummary(change updates.RouteChange, text pushText) string {
	if change.Source == updates.SourceStops {
		added, removed := make(map[string]bool), make(map[string]bool)
		for _, item := range change.Changes {
			for _, stop := range item.Stops {
				switch item.Kind {
				case updates.StopsAdded:
					added[stop] = true
				case updates.StopsRemoved:
					removed[stop] = true
				}
			}
		}
		switch {
		case len(added) > 0 && len(removed) > 0:
			return text.stopsBoth
		case len(added) == 1:
			return text.stopAdded
		case len(added) > 1:
			return text.stopsAdded
		case len(removed) == 1:
			return text.stopRemoved
		default:
			return text.stopsRemoved
		}
	}

	present := make(map[string]bool)
	for _, item := range change.Changes {
		for _, day := range item.Days {
			present[day] = true
		}
	}
	var days []string
	for _, day := range []string{"weekdays", "saturday", "sunday"} {
		if present[day] {
			days = append(days, text.days[day])
		}
	}
	switch len(days) {
	case 0:
		return text.timetableAny
	case 1:
		return fmt.Sprintf(text.timetableOne, days[0])
	default:
		return fmt.Sprintf(text.timetableMany, strings.Join(days[:len(days)-1], ", ")+" "+text.and+" "+days[len(days)-1])
	}
}
