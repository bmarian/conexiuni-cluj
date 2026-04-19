package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Slot is a time-of-day window with a target polling interval. If Idle is set,
// it's used instead of Normal when subscribers < IdleThreshold.
type Slot struct {
	StartSec      int // seconds since local midnight, inclusive
	EndSec        int // seconds since local midnight, exclusive (86400 for end-of-day)
	Normal        time.Duration
	Idle          time.Duration
	IdleThreshold int
}

func (s Slot) target(subscribers int) time.Duration {
	if s.Idle > 0 && subscribers < s.IdleThreshold {
		return s.Idle
	}
	return s.Normal
}

func (s Slot) label() string {
	return fmt.Sprintf("%s-%s", fmtHM(s.StartSec), fmtHM(s.EndSec))
}

func fmtHM(sec int) string {
	return fmt.Sprintf("%02d:%02d", sec/3600, (sec%3600)/60)
}

type VehicleIntervalConfig struct {
	Weekday     []Slot
	Weekend     []Slot
	MinInterval time.Duration
	MaxInterval time.Duration
}

func (c VehicleIntervalConfig) slotsFor(localNow time.Time) []Slot {
	d := localNow.Weekday()
	if d == time.Saturday || d == time.Sunday {
		return c.Weekend
	}
	return c.Weekday
}

// SlotAt returns the slot containing localNow, or (Slot{}, false) if none matches.
func (c VehicleIntervalConfig) SlotAt(localNow time.Time) (Slot, bool) {
	s := localNow.Hour()*3600 + localNow.Minute()*60 + localNow.Second()
	for _, sl := range c.slotsFor(localNow) {
		if s >= sl.StartSec && s < sl.EndSec {
			return sl, true
		}
	}
	return Slot{}, false
}

// ParseSchedule parses a schedule spec into an ordered list of slots.
// Format: "HH:MM-HH:MM;NORMAL[;IDLE@THRESHOLD]" entries separated by commas.
// Example: "00:00-06:00;30s;60s@20, 06:00-07:00;20s, 07:00-09:00;10s"
func ParseSchedule(spec string) ([]Slot, error) {
	entries := strings.Split(spec, ",")
	slots := make([]Slot, 0, len(entries))
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		parts := strings.Split(e, ";")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("slot %q: expected 2 or 3 ;-separated fields", e)
		}
		start, end, err := parseRange(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("slot %q: %w", e, err)
		}
		normal, err := time.ParseDuration(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("slot %q: normal interval: %w", e, err)
		}
		sl := Slot{StartSec: start, EndSec: end, Normal: normal}
		if len(parts) == 3 {
			idle, thr, err := parseIdle(strings.TrimSpace(parts[2]))
			if err != nil {
				return nil, fmt.Errorf("slot %q: %w", e, err)
			}
			sl.Idle = idle
			sl.IdleThreshold = thr
		}
		slots = append(slots, sl)
	}
	for i := 1; i < len(slots); i++ {
		if slots[i].StartSec < slots[i-1].EndSec {
			return nil, fmt.Errorf("slot %s overlaps previous %s", slots[i].label(), slots[i-1].label())
		}
	}
	return slots, nil
}

func parseRange(s string) (int, int, error) {
	dash := strings.IndexByte(s, '-')
	if dash < 0 {
		return 0, 0, fmt.Errorf("range %q missing '-'", s)
	}
	start, err := parseHM(s[:dash])
	if err != nil {
		return 0, 0, err
	}
	end, err := parseHM(s[dash+1:])
	if err != nil {
		return 0, 0, err
	}
	if end <= start {
		return 0, 0, fmt.Errorf("range %q: end must be after start", s)
	}
	return start, end, nil
}

func parseHM(s string) (int, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("time %q: expected HH:MM", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 24 {
		return 0, fmt.Errorf("time %q: invalid hour", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q: invalid minute", s)
	}
	sec := h*3600 + m*60
	if sec > 86400 {
		return 0, fmt.Errorf("time %q: past 24:00", s)
	}
	return sec, nil
}

func parseIdle(s string) (time.Duration, int, error) {
	at := strings.IndexByte(s, '@')
	if at < 0 {
		return 0, 0, fmt.Errorf("idle %q: expected DURATION@THRESHOLD", s)
	}
	d, err := time.ParseDuration(strings.TrimSpace(s[:at]))
	if err != nil {
		return 0, 0, fmt.Errorf("idle %q: duration: %w", s, err)
	}
	thr, err := strconv.Atoi(strings.TrimSpace(s[at+1:]))
	if err != nil || thr < 0 {
		return 0, 0, fmt.Errorf("idle %q: threshold: %v", s, err)
	}
	return d, thr, nil
}

// ComputeVehicleInterval picks a polling interval from the current slot's target
// and smoothly scales it by projected-spend / quota-remaining so the day's budget
// is spread out. Scaling is clamped to [MinInterval, MaxInterval].
func ComputeVehicleInterval(cfg VehicleIntervalConfig, subscribers, quotaRemaining int, now time.Time, loc *time.Location) time.Duration {
	localNow := now.In(loc)

	if quotaRemaining <= 0 {
		return cfg.MaxInterval
	}

	slots := cfg.slotsFor(localNow)
	sec := localNow.Hour()*3600 + localNow.Minute()*60 + localNow.Second()
	curIdx := -1
	for i, sl := range slots {
		if sec >= sl.StartSec && sec < sl.EndSec {
			curIdx = i
			break
		}
	}
	if curIdx < 0 {
		return cfg.MaxInterval
	}
	cur := slots[curIdx]
	target := cur.target(subscribers)

	// Project spend from now through end of day, assuming Normal for future slots.
	projected := float64(cur.EndSec-sec) / target.Seconds()
	for i := curIdx + 1; i < len(slots); i++ {
		sl := slots[i]
		projected += float64(sl.EndSec-sl.StartSec) / sl.Normal.Seconds()
	}
	if projected <= 0 {
		return cfg.MaxInterval
	}

	ratio := projected / float64(quotaRemaining)
	interval := time.Duration(float64(target) * ratio)

	if interval < cfg.MinInterval {
		interval = cfg.MinInterval
	}
	if interval > cfg.MaxInterval {
		interval = cfg.MaxInterval
	}
	return interval
}
