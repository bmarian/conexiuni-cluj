package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/gofiber/fiber/v3"
)

const (
	leaveBuffer = 5 * time.Minute
	// How early to get ready when the walk isn't known.
	noLocationLead = 20 * time.Minute
	// The heads-up comes this long before it's time to leave.
	reminderHeadsUp = 10 * time.Minute
	// A walk measured longer ago than this is taken as unknown.
	walkFreshFor = time.Hour
	// Closer than this to the time to leave, it says to go now.
	leaveSlack          = 2 * time.Minute
	clockFromMinutes    = 60
	reminderCheckEvery  = 30 * time.Second
	maxRemindersPerSub  = 20
	maxReminderTextLen  = 120
	maxReminderShortLen = 32
	// Minutes after midnight; can exceed 1440.
	maxReminderSlot = 1440 + 180
	maxReminderWalk = 1440
)

var (
	reminderIDPattern   = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
	reminderTripPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)
	reminderDayTypes    = []string{"weekday", "saturday", "sunday"}
)

type pushReminder struct {
	ID          string         `json:"id"`
	StopID      int            `json:"stop_id"`
	StopName    string         `json:"stop_name"`
	RouteID     int            `json:"route_id"`
	TripID      string         `json:"trip_id"`
	Line        string         `json:"line"`
	Destination string         `json:"destination"`
	Days        []int          `json:"days"`
	Date        string         `json:"date"`
	Slots       map[string]int `json:"slots"`
	// Minutes to the stop from where it was set; nil when unknown.
	Walk *int `json:"walk,omitempty"`
	// Unix ms when the walk was measured.
	WalkAt int64 `json:"walk_at,omitempty"`
}

type reminderSyncRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	Locale    string         `json:"locale"`
	Reminders []pushReminder `json:"reminders"`
}

type storedReminder struct {
	pushReminder
	sub         pushSubscriber
	lastFiredAt int64
	// Unix ms of the told bus's time to leave, until it's sent.
	leaveAt int64
}

// What a reminder says: when to go by, to go now, or, without a fresh walk or at the stop, to get ready.
type reminderKind int

const (
	reminderLeaveBy reminderKind = iota
	reminderLeaveNow
	reminderGetReady
)

type reminderText struct {
	leaveBy, leaveNow, getReady, in, at, walk string
}

var reminderTexts = map[string]reminderText{
	"ro": {
		leaveBy: "Trebuie să te pornești înainte de %s pentru a prinde autobuzul", leaveNow: "Pleacă acum",
		getReady: "Autobuzul tău pleacă la %s, pregătește-te",
		in:       "%s peste %d min", at: "%s la %s", walk: "%s · %d min de mers",
	},
	"en": {
		leaveBy: "Leave by %s to catch your bus", leaveNow: "Leave now",
		getReady: "Your bus leaves at %s, get ready",
		in:       "%s in %d min", at: "%s at %s", walk: "%s · %d min walk",
	},
}

// How long before the bus to set off.
func reminderLead(walk *int) time.Duration {
	if walk == nil {
		return noLocationLead
	}
	return time.Duration(*walk)*time.Minute + leaveBuffer
}

// How long before the bus a reminder first speaks: the heads-up before the walk, or the flat lead without a walk or at the stop.
func tellLead(walk *int) time.Duration {
	if walk == nil || *walk == 0 {
		return noLocationLead
	}
	return reminderLead(walk) + reminderHeadsUp
}

// The walk while it's fresh.
func (r pushReminder) walkNow(now time.Time) *int {
	if r.Walk == nil || now.Sub(time.UnixMilli(r.WalkAt)) > walkFreshFor {
		return nil
	}
	return r.Walk
}

func validReminder(r pushReminder) bool {
	if !reminderIDPattern.MatchString(r.ID) || r.StopID <= 0 || r.RouteID <= 0 {
		return false
	}
	if !reminderTripPattern.MatchString(r.TripID) || r.Line == "" || len(r.Line) > maxReminderShortLen {
		return false
	}
	if r.StopName == "" || len(r.StopName) > maxReminderTextLen || len(r.Destination) > maxReminderTextLen {
		return false
	}
	if len(r.Slots) == 0 || r.WalkAt < 0 || (r.Walk != nil && (*r.Walk < 0 || *r.Walk > maxReminderWalk)) {
		return false
	}
	for dayType, minute := range r.Slots {
		known := false
		for _, d := range reminderDayTypes {
			known = known || d == dayType
		}
		if !known || minute < 0 || minute >= maxReminderSlot {
			return false
		}
	}
	if r.Date != "" {
		if _, err := time.Parse("2006-01-02", r.Date); err != nil || len(r.Days) > 0 {
			return false
		}
		return true
	}
	if len(r.Days) == 0 || len(r.Days) > 7 {
		return false
	}
	seen := map[int]bool{}
	for _, d := range r.Days {
		if d < 1 || d > 7 || seen[d] {
			return false
		}
		seen[d] = true
	}
	return true
}

func (city *City) SyncReminders(c fiber.Ctx) error {
	if pushCfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "push notifications are not configured"})
	}
	if !sameOriginRequest(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	var req reminderSyncRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if !validPushEndpoint(req.Endpoint) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported push endpoint"})
	}
	if !validPushKey(req.Keys.P256dh, 65) || !validPushKey(req.Keys.Auth, 16) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid subscription keys"})
	}
	if len(req.Reminders) > maxRemindersPerSub {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "too many reminders"})
	}
	ids := map[string]bool{}
	for _, r := range req.Reminders {
		if !validReminder(r) || ids[r.ID] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid reminder"})
		}
		ids[r.ID] = true
	}
	if ok, err := city.admitPushEndpoint(ClientHashFromLocals(c), req.Endpoint, time.Now()); err != nil {
		city.log("push: could not look up subscription: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not save reminders"})
	} else if !ok {
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	locale := "ro"
	if req.Locale == "en" {
		locale = "en"
	}
	sub := pushSubscriber{endpoint: req.Endpoint, p256dh: req.Keys.P256dh, auth: req.Keys.Auth, locale: locale, host: c.Hostname()}
	if err := city.saveReminders(sub, req.Reminders, time.Now()); err != nil {
		city.log("push: could not save reminders: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not save reminders"})
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func (city *City) saveReminders(sub pushSubscriber, reminders []pushReminder, now time.Time) error {
	tx, err := city.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO push_subscriptions (endpoint, p256dh, auth, locale, host, updated_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
			p256dh = excluded.p256dh, auth = excluded.auth, locale = excluded.locale, host = excluded.host, updated_at = excluded.updated_at`,
		sub.endpoint, sub.p256dh, sub.auth, sub.locale, sub.host, time.Now().UnixMilli()); err != nil {
		return err
	}
	fired := map[string][2]int64{}
	rows, err := tx.Query(`SELECT id, last_fired_at, leave_at FROM push_reminders WHERE endpoint = ?`, sub.endpoint)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var at, leave int64
		if err := rows.Scan(&id, &at, &leave); err != nil {
			rows.Close()
			return err
		}
		fired[id] = [2]int64{at, leave}
	}
	rows.Close()
	if _, err := tx.Exec(`DELETE FROM push_reminders WHERE endpoint = ?`, sub.endpoint); err != nil {
		return err
	}
	for _, r := range reminders {
		days := make([]string, len(r.Days))
		for i, d := range r.Days {
			days[i] = strconv.Itoa(d)
		}
		slots, err := json.Marshal(r.Slots)
		if err != nil {
			return err
		}
		told, leaveAt := fired[r.ID][0], fired[r.ID][1]
		// Set after its heads-up was due: that moment has passed, only the time to leave is still ahead.
		if _, known := fired[r.ID]; !known {
			if step, ok := city.reminderStep(r, now); ok && step.kind == reminderLeaveBy {
				told, leaveAt = step.slot.UnixMilli(), step.leaveAt.UnixMilli()
			}
		}
		if _, err := tx.Exec(`
			INSERT INTO push_reminders (endpoint, id, stop_id, stop_name, route_id, trip_id, line, destination, days, date, slots, walk, walk_at, last_fired_at, leave_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sub.endpoint, r.ID, r.StopID, r.StopName, r.RouteID, r.TripID, r.Line, r.Destination,
			strings.Join(days, ","), r.Date, string(slots), r.Walk, r.WalkAt, told, leaveAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (city *City) runReminderSender(loc *time.Location) {
	ticker := time.NewTicker(reminderCheckEvery)
	defer ticker.Stop()
	for {
		city.fireDueReminders(time.Now(), loc)
		<-ticker.C
	}
}

// Starts a day back: late buses run past midnight.
func (city *City) nextReminderBus(r pushReminder, after time.Time, loc *time.Location) (time.Time, bool) {
	local := after.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	isoDays := map[int]bool{}
	for _, d := range r.Days {
		isoDays[d] = true
	}
	for i := -1; i <= 8; i++ {
		day := today.AddDate(0, 0, i)
		if r.Date != "" && day.Format("2006-01-02") != r.Date {
			continue
		}
		iso := int(day.Weekday())
		if iso == 0 {
			iso = 7
		}
		if r.Date == "" && !isoDays[iso] {
			continue
		}
		minute, ok := r.Slots[city.scheduleDay(day)]
		if !ok {
			continue
		}
		at := time.Date(day.Year(), day.Month(), day.Day(), 0, minute, 0, 0, loc)
		if at.After(after) {
			return at, true
		}
	}
	return time.Time{}, false
}

func (city *City) loadReminders() ([]storedReminder, error) {
	return queryRows(city.DB, `
		SELECT r.endpoint, r.id, r.stop_id, r.stop_name, r.route_id, r.trip_id, r.line, r.destination, r.days, r.date, r.slots,
		       r.walk, r.walk_at, r.last_fired_at, r.leave_at, s.p256dh, s.auth, s.locale
		FROM push_reminders r
		JOIN push_subscriptions s ON s.endpoint = r.endpoint`,
		nil,
		func(rows *sql.Rows) (storedReminder, error) {
			var r storedReminder
			var days, slots string
			var walk sql.NullInt64
			if err := rows.Scan(&r.sub.endpoint, &r.ID, &r.StopID, &r.StopName, &r.RouteID, &r.TripID, &r.Line, &r.Destination,
				&days, &r.Date, &slots, &walk, &r.WalkAt, &r.lastFiredAt, &r.leaveAt, &r.sub.p256dh, &r.sub.auth, &r.sub.locale); err != nil {
				return r, err
			}
			if walk.Valid {
				minutes := int(walk.Int64)
				r.Walk = &minutes
			}
			for _, part := range strings.Split(days, ",") {
				if d, err := strconv.Atoi(part); err == nil {
					r.Days = append(r.Days, d)
				}
			}
			return r, json.Unmarshal([]byte(slots), &r.Slots)
		})
}

func (city *City) fireDueReminders(now time.Time, loc *time.Location) {
	reminders, err := city.loadReminders()
	if err != nil {
		city.log("push: could not load reminders: %v", err)
		return
	}
	sort.Slice(reminders, func(i, j int) bool { return reminders[i].ID < reminders[j].ID })
	for _, r := range reminders {
		if r.leaveAt > 0 && !now.Before(time.UnixMilli(r.leaveAt)) {
			city.sendLeaveNow(r, now, loc)
		}
		bus, ok := city.nextReminderBus(r.pushReminder, now, loc)
		if !ok {
			if r.Date != "" {
				if _, err := city.DB.Exec(`DELETE FROM push_reminders WHERE endpoint = ? AND id = ?`, r.sub.endpoint, r.ID); err != nil {
					city.log("push: could not drop a past reminder: %v", err)
				}
			}
			continue
		}
		tell, due := city.tellFor(r.pushReminder, bus, now)
		if !due || r.lastFiredAt >= tell.slot.UnixMilli() {
			continue
		}
		var pending int64
		if tell.kind == reminderLeaveBy {
			pending = tell.leaveAt.UnixMilli()
		}
		// Marked before sending, so a failed send can't repeat.
		if _, err := city.DB.Exec(`UPDATE push_reminders SET last_fired_at = ?, leave_at = ? WHERE endpoint = ? AND id = ?`,
			tell.slot.UnixMilli(), pending, r.sub.endpoint, r.ID); err != nil {
			city.log("push: could not mark a reminder as sent: %v", err)
			continue
		}
		city.sendReminder(r, reminderPushMessage(city.Slug, r.pushReminder, r.sub.locale, tell.kind, tell.walk, tell.slot, tell.bus, now, loc), tell.bus, now)
	}
}

type reminderTell struct {
	// The timetable time, and the live one when known.
	slot, bus time.Time
	kind      reminderKind
	walk      *int
	leaveAt   time.Time
}

// How the bus at slot is told at now; false while it's too early to tell.
func (city *City) tellFor(r pushReminder, slot, now time.Time) (reminderTell, bool) {
	tell := reminderTell{slot: slot, bus: slot, kind: reminderGetReady, walk: r.walkNow(now)}
	if at, ok := city.liveArrivalAt(r.TripID, r.StopID, slot); ok {
		tell.bus = at
	}
	if tell.bus.Sub(now) > tellLead(tell.walk) {
		return tell, false
	}
	if tell.walk != nil && *tell.walk > 0 {
		tell.kind, tell.leaveAt = reminderLeaveNow, tell.bus.Add(-reminderLead(tell.walk)).Truncate(time.Minute)
		if tell.leaveAt.Sub(now) > leaveSlack {
			tell.kind = reminderLeaveBy
		}
	}
	return tell, true
}

// The reminder's next bus, if it is already to be told.
func (city *City) reminderStep(r pushReminder, now time.Time) (reminderTell, bool) {
	slot, ok := city.nextReminderBus(r, now, city.Location)
	if !ok {
		return reminderTell{}, false
	}
	return city.tellFor(r, slot, now)
}

// The told bus's time to leave, with the walk it was timed from.
func (city *City) sendLeaveNow(r storedReminder, now time.Time, loc *time.Location) {
	if _, err := city.DB.Exec(`UPDATE push_reminders SET leave_at = 0 WHERE endpoint = ? AND id = ?`, r.sub.endpoint, r.ID); err != nil {
		city.log("push: could not mark a reminder's time to leave as sent: %v", err)
		return
	}
	slot := time.UnixMilli(r.lastFiredAt)
	bus := slot
	if at, ok := city.liveArrivalAt(r.TripID, r.StopID, slot); ok {
		bus = at
	}
	if !now.Before(bus) || r.Walk == nil {
		return
	}
	city.sendReminder(r, reminderPushMessage(city.Slug, r.pushReminder, r.sub.locale, reminderLeaveNow, r.Walk, slot, bus, now, loc), bus, now)
}

func (city *City) sendReminder(r storedReminder, message pushMessage, bus, now time.Time) {
	payload, err := json.Marshal(message)
	if err != nil {
		return
	}
	ttl := int(bus.Sub(now).Seconds())
	if ttl < 60 {
		ttl = 60
	}
	city.sendPushWith(r.sub, payload, ttl, webpush.UrgencyHigh)
}

// [slot] is the bus's timetable time, which a tap follows; [bus] its live time when known.
func reminderPushMessage(slug string, r pushReminder, locale string, kind reminderKind, walk *int, slot, bus, now time.Time, loc *time.Location) pushMessage {
	text, ok := reminderTexts[locale]
	if !ok {
		text = reminderTexts["ro"]
	}
	minutes := int((bus.Sub(now) + 30*time.Second) / time.Minute)
	if minutes < 0 {
		minutes = 0
	}
	coming := fmt.Sprintf(text.in, r.Line, minutes)
	if minutes >= clockFromMinutes {
		coming = fmt.Sprintf(text.at, r.Line, bus.In(loc).Format("15:04"))
	}
	title, body := fmt.Sprintf(text.getReady, bus.In(loc).Format("15:04")), coming
	if kind != reminderGetReady && walk != nil {
		body = fmt.Sprintf(text.walk, coming, *walk)
		title = text.leaveNow
		if kind == reminderLeaveBy {
			title = fmt.Sprintf(text.leaveBy, bus.Add(-reminderLead(walk)).Truncate(time.Minute).In(loc).Format("15:04"))
		}
	}
	return pushMessage{
		Title:     title,
		Body:      body,
		URL:       fmt.Sprintf("/%s/stop/%d?follow=%s&at=%d", slug, r.StopID, r.TripID, slot.UnixMilli()),
		Tag:       "reminder-" + r.ID,
		Timestamp: now.UnixMilli(),
		Alert:     kind != reminderLeaveBy,
	}
}
