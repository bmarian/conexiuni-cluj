package live

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/web"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

type SchoolCalendar interface {
	SchoolYears() ([]calendar.SchoolYear, error)
	Cadence() time.Duration
	Page() string
}

const fetchSchoolCalendar = "school_calendar"

type schoolSyncState struct {
	sync.Mutex
	err string
}

type adminCalendarSync struct {
	Source    string `json:"source"`
	CheckedAt int64  `json:"checked_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (eng *Engine) startSchoolCalendar(r *jobs.Runner) {
	source := eng.SchoolCalendar
	if source == nil || eng.Archive == nil {
		return
	}
	r.Go(func() {
		for {
			if !r.Sleep(time.Until(eng.FetchedAt(fetchSchoolCalendar).Add(source.Cadence()))) {
				return
			}
			if err := eng.syncSchoolCalendar(time.Now()); err != nil {
				eng.log("calendar: %v", err)
				if !r.Sleep(source.Cadence()) {
					return
				}
			}
		}
	})
}

func (eng *Engine) syncSchoolCalendar(now time.Time) error {
	eng.schoolSync.Lock()
	defer eng.schoolSync.Unlock()
	err := eng.storeSchoolCalendar(now)
	eng.schoolSync.err = ""
	if err != nil {
		eng.schoolSync.err = err.Error()
		return err
	}
	eng.MarkFetched(fetchSchoolCalendar, now)
	return eng.reloadCalendar(now)
}

// A hand-set year equal to the feed's is dropped so it follows the feed.
func (eng *Engine) storeSchoolCalendar(now time.Time) error {
	source := eng.SchoolCalendar
	years, err := source.SchoolYears()
	if err != nil {
		return err
	}
	synced, err := eng.syncedSchoolYears()
	if err != nil {
		return err
	}
	saved, err := eng.savedSchoolYears()
	if err != nil {
		return err
	}
	for _, y := range years {
		y = y.Normalized()
		if err := y.Validate(); err != nil {
			eng.log("calendar: %s has an invalid %d–%d school year: %v", source.Page(), y.Year, y.Year+1, err)
			continue
		}
		if old, ok := synced[y.Year]; !ok || !old.Equal(y) {
			data, err := json.Marshal(y)
			if err != nil {
				return err
			}
			if err := eng.SaveSyncedSchoolYear(y.Year, string(data), now); err != nil {
				return err
			}
			eng.log("calendar: school year %d–%d read from %s", y.Year, y.Year+1, source.Page())
		}
		if own, ok := saved[y.Year]; ok && own.Equal(y) {
			if err := eng.DeleteSchoolYear(y.Year); err != nil {
				return err
			}
			eng.log("calendar: school year %d–%d set on the admin page matches %s, following it from now on", y.Year, y.Year+1, source.Page())
		}
	}
	return nil
}

func (eng *Engine) reloadCalendar(now time.Time) error {
	before := eng.currentCalendar().Years()
	if err := eng.LoadCalendar(); err != nil {
		return err
	}
	if slices.EqualFunc(before, eng.currentCalendar().Years(), calendar.SchoolYear.Equal) {
		return nil
	}
	eng.invalidateScheduleCaches()
	relearnAfterCalendarChange(eng, now)
	return nil
}

func (eng *Engine) syncedSchoolYears() (map[int]calendar.SchoolYear, error) {
	return decodeSchoolYears(eng.SyncedSchoolYears())
}

func (eng *Engine) syncStatus() *adminCalendarSync {
	if eng.SchoolCalendar == nil {
		return nil
	}
	s := &adminCalendarSync{Source: eng.SchoolCalendar.Page()}
	if at := eng.FetchedAt(fetchSchoolCalendar); !at.IsZero() {
		s.CheckedAt = at.Unix()
	}
	eng.schoolSync.Lock()
	s.Error = eng.schoolSync.err
	eng.schoolSync.Unlock()
	return s
}

func (eng *Engine) saveAdminSchoolYear(year calendar.SchoolYear, now time.Time) error {
	synced, err := eng.syncedSchoolYears()
	if err != nil {
		return err
	}
	if s, ok := synced[year.Year]; ok && s.Equal(year) {
		eng.log("calendar: school year %d–%d follows the school calendar feed again", year.Year, year.Year+1)
		return eng.DeleteSchoolYear(year.Year)
	}
	data, err := json.Marshal(year)
	if err != nil {
		return err
	}
	return eng.SaveSchoolYear(year.Year, string(data), now)
}

func syncSchoolCalendarHandler(cityOf func(fiber.Ctx) *Engine) fiber.Handler {
	return func(c fiber.Ctx) error {
		eng := cityOf(c)
		if !web.SameOrigin(c) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		if eng.SchoolCalendar == nil || eng.Archive == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "this city has no school calendar to read"})
		}
		now := time.Now()
		if err := eng.syncSchoolCalendar(now); err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": fmt.Sprintf("could not read %s: %v", eng.SchoolCalendar.Page(), err)})
		}
		view, err := eng.buildAdminCalendar(now)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(view)
	}
}
