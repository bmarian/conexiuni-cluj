package handlers

import (
	"bytes"
	"conexiuni-cluj/database"
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

var (
	pushCfg    *pushConfig
	pushWake   chan struct{}
	pushClient = &http.Client{Timeout: 15 * time.Second}
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
		timetableOne:  "Sunt schimbări la orarul %s.",
		timetableMany: "Sunt schimbări la orarul %s.",
		timetableAny:  "Sunt schimbări la orar.",
		stopAdded:     "O stație nouă a fost adăugată pe traseu.",
		stopsAdded:    "Au fost adăugate stații noi pe traseu.",
		stopRemoved:   "O stație a fost scoasă de pe traseu.",
		stopsRemoved:  "Unele stații au fost scoase de pe traseu.",
		stopsBoth:     "Unele stații au fost adăugate pe traseu, iar altele au fost scoase.",
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
		timetableOne:  "There are changes to the %s timetable.",
		timetableMany: "There are changes to the %s timetables.",
		timetableAny:  "There are changes to the timetable.",
		stopAdded:     "A new stop was added to the route.",
		stopsAdded:    "New stops were added to the route.",
		stopRemoved:   "A stop was removed from the route.",
		stopsRemoved:  "Some stops were removed from the route.",
		stopsBoth:     "Some stops were added to the route and others were removed.",
	},
}

type pushMessage struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Tag       string `json:"tag"`
	Timestamp int64  `json:"timestamp"`
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

func InitPush(publicKey, privateKey, subject string) error {
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
		subject = siteOrigin
	}

	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		log.Printf("push: could not load Europe/Bucharest, using local time: %v", err)
		loc = time.Local
	}

	pushCfg = &pushConfig{publicKey: publicKey, privateKey: privateKey, subject: subject}
	pushWake = make(chan struct{}, 1)
	go runPushSender(loc)
	log.Printf("push: notifications enabled, sent between %02d:00 and %02d:00", pushWindowStartHour, pushWindowEndHour)
	return nil
}

// Changes found overnight (the 04:00 warmup, a deploy) wait for the morning instead of waking people up.
func runPushSender(loc *time.Location) {
	for {
		now := time.Now().In(loc)
		if !inPushWindow(now) {
			time.Sleep(time.Until(nextPushWindow(now)))
			continue
		}
		flushPendingPushes()
		<-pushWake
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

func GetPushKey(c fiber.Ctx) error {
	if pushCfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "push notifications are not configured"})
	}
	c.Set("Cache-Control", revalidateCacheControl)
	return c.JSON(fiber.Map{"public_key": pushCfg.publicKey})
}

func SubscribePush(c fiber.Ctx) error {
	if pushCfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "push notifications are not configured"})
	}
	if !sameOriginRequest(c) {
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
	locale := "ro"
	if req.Locale == "en" {
		locale = "en"
	}
	sub := pushSubscriber{endpoint: req.Endpoint, p256dh: req.Keys.P256dh, auth: req.Keys.Auth, locale: locale}
	if err := savePushSubscription(sub, cleanPushRouteNames(req.Routes)); err != nil {
		log.Printf("push: could not save subscription: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not save subscription"})
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func UnsubscribePush(c fiber.Ctx) error {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := c.Bind().Body(&req); err != nil || req.Endpoint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := deletePushSubscription(req.Endpoint); err != nil {
		log.Printf("push: could not delete subscription: %v", err)
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
		if len(out) == maxRouteChangeFilterNames {
			break
		}
	}
	return out
}

func savePushSubscription(sub pushSubscriber, routes []string) error {
	tx, err := database.DB.Begin()
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

func deletePushSubscription(endpoint string) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM push_subscription_routes WHERE endpoint = ?`, endpoint); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint); err != nil {
		return err
	}
	return tx.Commit()
}

func wakePushSender() {
	if pushWake == nil {
		return
	}
	select {
	case pushWake <- struct{}{}:
	default:
	}
}

func flushPendingPushes() {
	changes, err := queryRows(`
		SELECT id, route_short_name, source, detected_at, changes
		FROM route_changes
		WHERE push_pending = 1
		ORDER BY id DESC`,
		nil,
		func(rows *sql.Rows) (RouteChange, error) {
			var rc RouteChange
			var payload string
			if err := rows.Scan(&rc.ID, &rc.RouteShortName, &rc.Source, &rc.DetectedAt, &payload); err != nil {
				return rc, err
			}
			return rc, json.Unmarshal([]byte(payload), &rc.Changes)
		})
	if err != nil {
		log.Printf("push: could not load pending changes: %v", err)
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
			sendRouteChangePush(change)
		}
	}
	if _, err := database.DB.Exec(`UPDATE route_changes SET push_pending = 0 WHERE push_pending = 1 AND id <= ?`,
		changes[0].ID); err != nil {
		log.Printf("push: could not mark changes as sent: %v", err)
	}
}

func sendRouteChangePush(change RouteChange) {
	subscribers, err := queryRows(`
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
		log.Printf("push: could not load followers of line %s: %v", change.RouteShortName, err)
		return
	}
	if len(subscribers) == 0 {
		return
	}
	if change.RouteID == 0 {
		_ = database.DB.QueryRow(`SELECT COALESCE(MAX(route_id), 0) FROM routes WHERE route_short_name = ?`,
			change.RouteShortName).Scan(&change.RouteID)
	}

	delivered := 0
	for _, sub := range subscribers {
		payload, err := json.Marshal(routeChangePushMessage(change, sub.locale))
		if err != nil {
			log.Printf("push: could not encode line %s change: %v", change.RouteShortName, err)
			return
		}
		if sendPush(sub, payload) {
			delivered++
		}
	}
	log.Printf("push: line %s change %d sent to %d/%d followers", change.RouteShortName, change.ID, delivered, len(subscribers))
}

func sendPush(sub pushSubscriber, payload []byte) bool {
	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.endpoint,
		Keys:     webpush.Keys{P256dh: sub.p256dh, Auth: sub.auth},
	}, &webpush.Options{
		HTTPClient:      pushClient,
		RecordSize:      pushRecordSize,
		Subscriber:      pushCfg.subject,
		TTL:             pushTTLSeconds,
		Urgency:         webpush.UrgencyNormal,
		VAPIDPublicKey:  pushCfg.publicKey,
		VAPIDPrivateKey: pushCfg.privateKey,
	})
	host := pushEndpointHost(sub.endpoint)
	if err != nil {
		log.Printf("push: sending to %s failed: %v", host, err)
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		if err := deletePushSubscription(sub.endpoint); err != nil {
			log.Printf("push: could not drop expired subscription on %s: %v", host, err)
		}
	default:
		log.Printf("push: %s rejected a notification with status %d: %s", host, resp.StatusCode, strings.TrimSpace(string(body)))
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

func routeChangePushMessage(change RouteChange, locale string) pushMessage {
	text, ok := pushTexts[locale]
	if !ok {
		text = pushTexts["ro"]
	}
	what := text.timetable
	if change.Source == routeChangeSourceStops {
		what = text.stops
	}
	target := "/"
	if change.RouteID != 0 {
		direction := "0"
		if len(change.Changes) > 0 && change.Changes[0].Direction != "" {
			direction = change.Changes[0].Direction
		}
		target = fmt.Sprintf("/route/%d/%s?change=%d", change.RouteID, direction, change.ID)
	}
	return pushMessage{
		Title:     fmt.Sprintf(text.line, change.RouteShortName, what),
		Body:      routeChangeSummary(change, text),
		URL:       target,
		Tag:       "route-change-" + change.RouteShortName,
		Timestamp: change.DetectedAt,
	}
}

func routeChangeSummary(change RouteChange, text pushText) string {
	if change.Source == routeChangeSourceStops {
		added, removed := make(map[string]bool), make(map[string]bool)
		for _, item := range change.Changes {
			for _, stop := range item.Stops {
				switch item.Kind {
				case changeStopsAdded:
					added[stop] = true
				case changeStopsRemoved:
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
