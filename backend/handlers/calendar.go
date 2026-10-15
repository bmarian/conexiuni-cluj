package handlers

import (
	"cmp"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

var dayWindows = struct {
	sync.RWMutex
	days map[string]int
}{days: map[string]int{
	calendar.Weekday:         10,
	calendar.WeekdayVacation: 5,
	calendar.Saturday:        2,
	calendar.Sunday:          2,
}}

func dayWindowSize(dayType string) (int, bool) {
	dayWindows.RLock()
	defer dayWindows.RUnlock()
	n, ok := dayWindows.days[dayType]
	return n, ok
}

func setDayWindowSize(dayType string, n int) {
	dayWindows.Lock()
	dayWindows.days[dayType] = n
	dayWindows.Unlock()
	calendarGeneration.Add(1)
}

const (
	legacyDayWindow    = 14 * 24 * time.Hour
	dayWindowMaxReach  = 400
	calendarApplyDelay = time.Minute
	observedMinRuns    = 150
	observedMinFit     = 0.5
	observedMinLead    = 0.1
	observedEdgeMin    = 10.0
	adminOverrideDays  = 90
	adminHolidayDays   = 365
	adminDayReach      = 400
	overrideByAdmin    = "admin"
	overrideObserved   = "observed"
	dayNoteHoliday     = "holiday"
	dayNoteSet         = "set"
	dayNoteObserved    = "observed"
)

var (
	activeCalendar     atomic.Pointer[calendar.Calendar]
	calendarGeneration atomic.Int64
	applyCalendarMu    sync.Mutex

	relearnAfterCalendarChange = func(now time.Time) { go applyCalendar(now) }

	dayWindowCache = struct {
		sync.Mutex
		key     string
		windows map[string]dayWindow
	}{}

	seedFills = struct {
		sync.RWMutex
		fills map[string]string
	}{fills: map[string]string{}}
)

const (
	runMaxGapDays     = 14
	lastYearReachDays = 60
	seedPrevious      = "previous"
	seedLastYear      = "last_year"
)

func seedFill(dayType string) string {
	seedFills.RLock()
	defer seedFills.RUnlock()
	return cmp.Or(seedFills.fills[dayType], seedPrevious)
}

func setSeedFill(dayType, fill string) {
	seedFills.Lock()
	seedFills.fills[dayType] = fill
	seedFills.Unlock()
	calendarGeneration.Add(1)
}

func currentCalendar() *calendar.Calendar {
	if c := activeCalendar.Load(); c != nil {
		return c
	}
	activeCalendar.CompareAndSwap(nil, calendar.New(calendar.Seed(), nil))
	return activeCalendar.Load()
}

func setCalendar(c *calendar.Calendar) {
	activeCalendar.Store(c)
	calendarGeneration.Add(1)
}

func profileDay(t time.Time) string  { return currentCalendar().ProfileDay(t) }
func scheduleDay(t time.Time) string { return currentCalendar().ScheduleDay(t) }

func StartCalendar() {
	if err := LoadCalendar(); err != nil {
		log.Printf("calendar: load failed, using the built-in school year: %v", err)
	}
	LoadWindowTuning()
	go func() {
		time.Sleep(calendarApplyDelay)
		applyCalendar(time.Now())
	}()
}

func LoadCalendar() error {
	years := map[int]calendar.SchoolYear{}
	for _, y := range calendar.Seed() {
		years[y.Year] = y
	}
	saved, err := savedSchoolYears()
	if err != nil {
		return err
	}
	for year, y := range saved {
		years[year] = y
	}
	overrides, err := database.ScheduleOverrides()
	if err != nil {
		return err
	}
	byDate := make(map[string]string, len(overrides))
	for _, o := range overrides {
		byDate[o.Date] = o.Schedule
	}
	list := make([]calendar.SchoolYear, 0, len(years))
	for _, y := range years {
		list = append(list, y)
	}
	setCalendar(calendar.New(list, byDate))
	return nil
}

func savedSchoolYears() (map[int]calendar.SchoolYear, error) {
	raw, err := database.SchoolYears()
	if err != nil {
		return nil, err
	}
	out := make(map[int]calendar.SchoolYear, len(raw))
	for year, data := range raw {
		var y calendar.SchoolYear
		if err := json.Unmarshal([]byte(data), &y); err != nil {
			return nil, fmt.Errorf("school year %d: %w", year, err)
		}
		out[year] = y
	}
	return out, nil
}

func localMidnight(t time.Time) time.Time {
	local := t.In(database.StatsLocation())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

type windowRange struct {
	from, to int64
}

type dayWindow struct {
	ranges   []windowRange
	fill     string
	runDays  int
	lastYear []time.Time
}

func (w dayWindow) contains(at int64) bool {
	for _, r := range w.ranges {
		if at >= r.from && at < r.to {
			return true
		}
	}
	return false
}

func (w dayWindow) sql(col string) (string, []any) {
	if len(w.ranges) == 0 {
		return "0", nil
	}
	parts := make([]string, 0, len(w.ranges))
	args := make([]any, 0, 2*len(w.ranges))
	for _, r := range w.ranges {
		if r.to == math.MaxInt64 {
			parts = append(parts, col+" >= ?")
			args = append(args, r.from)
			continue
		}
		parts = append(parts, "("+col+" >= ? AND "+col+" < ?)")
		args = append(args, r.from, r.to)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func (w dayWindow) earliest() int64 {
	earliest := int64(math.MaxInt64)
	for _, r := range w.ranges {
		earliest = min(earliest, r.from)
	}
	return earliest
}

func (w dayWindow) since() (int64, bool) {
	for _, r := range w.ranges {
		if r.to == math.MaxInt64 {
			return r.from, true
		}
	}
	return 0, false
}

func dayRun(cal *calendar.Calendar, dayType string, today time.Time, n int) []time.Time {
	var run []time.Time
	last := 0
	for i := 0; i < dayWindowMaxReach && len(run) < n && i-last <= runMaxGapDays; i++ {
		if d := today.AddDate(0, 0, -i); cal.ProfileDay(d) == dayType {
			run = append(run, d)
			last = i
		}
	}
	return run
}

func lastYearDays(cal *calendar.Calendar, dayType string, from time.Time, missing int) []time.Time {
	anchor := from.AddDate(-1, 0, 0)
	var out []time.Time
	for i := 0; i < lastYearReachDays && len(out) < missing; i++ {
		if d := anchor.AddDate(0, 0, i); cal.ProfileDay(d) == dayType {
			out = append(out, d)
		}
	}
	return out
}

func planDayWindow(cal *calendar.Calendar, dayType string, today time.Time, n int, fill string, covered func([]time.Time) bool) dayWindow {
	run := dayRun(cal, dayType, today, n)
	w := dayWindow{fill: seedPrevious, runDays: len(run)}
	if missing := n - len(run); missing > 0 && fill == seedLastYear {
		from := today
		if len(run) > 0 {
			from = run[len(run)-1]
		}
		if extra := lastYearDays(cal, dayType, from, missing); len(extra) > 0 && covered(extra) {
			w.fill, w.lastYear = seedLastYear, extra
			if len(run) > 0 {
				w.ranges = append(w.ranges, windowRange{run[len(run)-1].Unix(), math.MaxInt64})
			}
			w.ranges = append(w.ranges, windowRange{extra[0].Unix(), extra[len(extra)-1].AddDate(0, 0, 1).Unix()})
			return w
		}
	}
	start := today.AddDate(0, 0, -dayWindowMaxReach).Unix()
	if days := daysOfType(cal, today.AddDate(0, 0, 1), dayType, n); len(days) == n {
		start = days[n-1].Unix()
	}
	w.ranges = []windowRange{{start, math.MaxInt64}}
	return w
}

func archiveCovers(days []time.Time) bool {
	covered := 0
	for _, d := range days {
		ok, err := database.ArchiveHasSegmentSamples(d.Unix(), d.AddDate(0, 0, 1).Unix())
		if err != nil {
			log.Printf("calendar: archive check for %s failed: %v", d.Format("2006-01-02"), err)
			return false
		}
		if ok {
			covered++
		}
	}
	return 2*covered >= len(days)
}

func currentDayWindow(dayType string, now time.Time) dayWindow {
	n, ok := dayWindowSize(dayType)
	if !ok {
		return dayWindow{fill: seedPrevious, ranges: []windowRange{{now.Add(-legacyDayWindow).Unix(), math.MaxInt64}}}
	}
	today := localMidnight(now)
	key := today.Format("2006-01-02") + "|" + strconv.FormatInt(calendarGeneration.Load(), 10)
	dayWindowCache.Lock()
	defer dayWindowCache.Unlock()
	if dayWindowCache.key != key {
		dayWindowCache.key, dayWindowCache.windows = key, make(map[string]dayWindow)
	}
	if w, ok := dayWindowCache.windows[dayType]; ok {
		return w
	}
	w := planDayWindow(currentCalendar(), dayType, today, n, seedFill(dayType), archiveCovers)
	dayWindowCache.windows[dayType] = w
	return w
}

func earliestWindowStart(now time.Time) int64 {
	earliest := now.Add(-legacyDayWindow).Unix()
	for _, day := range calendar.ProfileDays {
		earliest = min(earliest, currentDayWindow(day, now).earliest())
	}
	return earliest
}

func pruneByDayWindow(table, timeCol string, retention time.Duration, now time.Time) (int64, error) {
	floor := now.Add(-retention).Unix()
	var pruned int64
	known := make([]any, 0, len(calendar.ProfileDays))
	for _, day := range calendar.ProfileDays {
		known = append(known, day)
		inWindow, args := currentDayWindow(day, now).sql(timeCol)
		res, err := database.DB.Exec(`DELETE FROM `+table+` WHERE day_type = ? AND `+timeCol+` < ? AND NOT `+inWindow,
			append([]any{day, floor}, args...)...)
		if err != nil {
			return pruned, err
		}
		n, _ := res.RowsAffected()
		pruned += n
	}
	res, err := database.DB.Exec(`DELETE FROM `+table+` WHERE day_type NOT IN (`+strings.TrimSuffix(strings.Repeat("?,", len(known)), ",")+`) AND `+timeCol+` < ?`,
		append(known, floor)...)
	if err != nil {
		return pruned, err
	}
	n, _ := res.RowsAffected()
	return pruned + n, nil
}

var labelledTables = []struct{ table, timeCol string }{
	{"segment_travel_time_samples", "observed_at"},
	{"turnaround_samples", "departed_at"},
}

func relabelSamples(now time.Time) (int64, error) {
	var total int64
	for _, t := range labelledTables {
		var oldest sql.NullInt64
		if err := database.DB.QueryRow(`SELECT MIN(` + t.timeCol + `) FROM ` + t.table).Scan(&oldest); err != nil {
			return total, err
		}
		if !oldest.Valid {
			continue
		}
		for day := localMidnight(time.Unix(oldest.Int64, 0)); !day.After(now); day = day.AddDate(0, 0, 1) {
			label := profileDay(day)
			res, err := database.DB.Exec(`UPDATE `+t.table+` SET day_type = ? WHERE `+t.timeCol+` >= ? AND `+t.timeCol+` < ? AND day_type != ?`,
				label, day.Unix(), day.AddDate(0, 0, 1).Unix(), label)
			if err != nil {
				return total, err
			}
			n, _ := res.RowsAffected()
			total += n
		}
	}
	return total, nil
}

func restoreWindowsFromArchive(now time.Time) (int, error) {
	if database.Archive == nil {
		return 0, nil
	}
	loc := database.StatsLocation()
	total := 0
	for _, t := range labelledTables {
		for _, day := range calendar.ProfileDays {
			for _, r := range restoreRanges(currentDayWindow(day, now), now) {
				var oldest sql.NullInt64
				if err := database.DB.QueryRow(`SELECT MIN(`+t.timeCol+`) FROM `+t.table+` WHERE day_type = ? AND `+t.timeCol+` >= ? AND `+t.timeCol+` < ?`,
					day, r.from, r.to).Scan(&oldest); err != nil {
					return total, err
				}
				until := r.to
				if oldest.Valid {
					until = oldest.Int64
				}
				n, err := database.RestoreFromArchive(t.table, r.from, until, func(at int64) (string, bool) {
					return day, profileDay(time.Unix(at, 0).In(loc)) == day
				})
				total += n
				if err != nil {
					return total, fmt.Errorf("%s: %w", t.table, err)
				}
			}
		}
	}
	return total, nil
}

func restoreRanges(w dayWindow, now time.Time) []windowRange {
	var out []windowRange
	if since, ok := w.since(); ok {
		out = append(out, windowRange{since, now.Unix()})
	}
	for _, d := range w.lastYear {
		out = append(out, windowRange{d.Unix(), d.AddDate(0, 0, 1).Unix()})
	}
	return out
}

func restoreSeededWindows(now time.Time) {
	seeded := false
	for _, day := range calendar.ProfileDays {
		seeded = seeded || len(currentDayWindow(day, now).lastYear) > 0
	}
	if !seeded || database.DB == nil {
		return
	}
	applyCalendarMu.Lock()
	restored, err := restoreWindowsFromArchive(now)
	applyCalendarMu.Unlock()
	if err != nil {
		log.Printf("calendar: restoring last year's days failed: %v", err)
	}
	if restored > 0 {
		log.Printf("calendar: restored %d samples of last year's days; relearning", restored)
		applyCalendar(now)
	}
}

func applyCalendar(now time.Time) {
	applyCalendarMu.Lock()
	defer applyCalendarMu.Unlock()
	if database.DB == nil {
		return
	}
	started := time.Now()
	relabelled, err := relabelSamples(now)
	if err != nil {
		log.Printf("calendar: relabel failed: %v", err)
	}
	restored, err := restoreWindowsFromArchive(now)
	if err != nil {
		log.Printf("calendar: restore from the archive failed: %v", err)
	}
	rebuilt, err := rebuildSegmentProfiles(now)
	if err != nil {
		log.Printf("calendar: profile rebuild failed: %v", err)
	}
	resetProfileSnapshots()
	resetTripScheduleCache()
	delays, err := recomputeScheduleDelays(database.StatsLocation(), now)
	if err != nil {
		log.Printf("calendar: schedule delay failed: %v", err)
	}
	invalidateScheduleCaches()
	log.Printf("calendar: relabelled %d samples, restored %d from the archive, rebuilt %d profiles and %d delays in %s",
		relabelled, restored, rebuilt, delays, time.Since(started).Round(time.Millisecond))
}

func invalidateScheduleCaches() {
	resetProfileSnapshots()
	resetTripScheduleCache()
	networkCache.Lock()
	if networkCache.current != nil {
		stale := *networkCache.current
		stale.period = -1
		networkCache.current = &stale
	}
	networkCache.Unlock()
	planner.Lock()
	planner.networks = nil
	planner.Unlock()
	go refreshLiveTrips(true)
}

type scheduleFit struct {
	Day      string
	Fit      float64
	Matched  int
	Expected int
}

func observedScheduleFits(day time.Time) ([]scheduleFit, int, error) {
	start := localMidnight(day)
	end := start.AddDate(0, 0, 1)

	type tripStop struct {
		trip string
		stop int
	}
	stops, err := queryRows(`SELECT trip_id, stop_id FROM stop_times ORDER BY trip_id, stop_sequence`, nil,
		func(rows *sql.Rows) (tripStop, error) {
			var s tripStop
			err := rows.Scan(&s.trip, &s.stop)
			return s, err
		})
	if err != nil {
		return nil, 0, err
	}
	type tripEnds struct{ first, second int }
	firstStops := map[string]*tripEnds{}
	for _, s := range stops {
		if ends, ok := firstStops[s.trip]; !ok {
			firstStops[s.trip] = &tripEnds{first: s.stop, second: -1}
		} else if ends.second < 0 {
			ends.second = s.stop
		}
	}

	type namedRoute struct {
		id   int
		name string
	}
	routes, err := queryRows(`SELECT route_id, route_short_name FROM routes`, nil, func(rows *sql.Rows) (namedRoute, error) {
		var r namedRoute
		err := rows.Scan(&r.id, &r.name)
		return r, err
	})
	if err != nil {
		return nil, 0, err
	}
	routeNames := make(map[int]string, len(routes))
	for _, r := range routes {
		routeNames[r.id] = r.name
	}

	type run struct {
		route, direction, from, to int
		departed                   float64
	}
	samples, err := queryRows(`
		SELECT route_id, direction_id, from_stop_id, to_stop_id, observed_at - duration_sec
		FROM segment_travel_time_samples
		WHERE observed_at >= ? AND observed_at < ?`,
		[]any{start.Unix(), end.Add(time.Hour).Unix()},
		func(rows *sql.Rows) (run, error) {
			var r run
			err := rows.Scan(&r.route, &r.direction, &r.from, &r.to, &r.departed)
			return r, err
		})
	if err != nil {
		return nil, 0, err
	}
	runs := map[[2]int][]float64{}
	for _, s := range samples {
		suffix := OUTGOING_SUFFIX
		if s.direction == 1 {
			suffix = INCOMING_SUFFIX
		}
		ends, ok := firstStops[strconv.Itoa(s.route)+suffix]
		if !ok || s.from != ends.first || s.to != ends.second {
			continue
		}
		minute := (s.departed - float64(start.Unix())) / 60
		if minute < 0 || minute >= 1440 {
			continue
		}
		runs[[2]int{s.route, s.direction}] = append(runs[[2]int{s.route, s.direction}], minute)
	}

	fits := map[string]*scheduleFit{}
	for _, d := range calendar.ScheduleDays {
		fits[d] = &scheduleFit{Day: d}
	}
	observed := 0
	for key, minutes := range runs {
		timetable, err := getTimetableFromDB(routeNames[key[0]])
		if err != nil || timetable == nil {
			continue
		}
		sort.Float64s(minutes)
		observed += len(minutes)
		lo, hi := minutes[0]-observedEdgeMin, minutes[len(minutes)-1]+observedEdgeMin
		for _, d := range calendar.ScheduleDays {
			departures := scheduleDepartureMinutes(timetable, d, key[1])
			fits[d].Matched += matchRuns(minutes, departures)
			for _, dep := range departures {
				if dep >= lo && dep <= hi {
					fits[d].Expected++
				}
			}
		}
	}
	out := make([]scheduleFit, 0, len(fits))
	for _, d := range calendar.ScheduleDays {
		f := fits[d]
		if observed+f.Expected > 0 {
			f.Fit = 2 * float64(f.Matched) / float64(observed+f.Expected)
		}
		out = append(out, *f)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Fit > out[j].Fit })
	return out, observed, nil
}

func matchRuns(observed, departures []float64) int {
	used := make([]bool, len(departures))
	matched := 0
	for _, o := range observed {
		best := -1
		for i, d := range departures {
			if !used[i] && (best < 0 || math.Abs(o-d) < math.Abs(o-departures[best])) {
				best = i
			}
		}
		if best < 0 {
			continue
		}
		gap := math.Inf(1)
		if best > 0 {
			gap = departures[best] - departures[best-1]
		}
		if best+1 < len(departures) {
			gap = math.Min(gap, departures[best+1]-departures[best])
		}
		if math.Abs(o-departures[best]) <= math.Min(scheduleDelayMaxMatchMin, 0.4*gap) {
			used[best] = true
			matched++
		}
	}
	return matched
}

func describeFits(fits []scheduleFit, observed int) string {
	parts := make([]string, len(fits))
	for i, f := range fits {
		parts[i] = fmt.Sprintf("%s %.2f", f.Day, f.Fit)
	}
	return fmt.Sprintf("%s over %d runs", strings.Join(parts, ", "), observed)
}

func checkObservedSchedule(now time.Time) {
	if database.Archive == nil {
		return
	}
	day := localMidnight(now).AddDate(0, 0, -1)
	date := day.Format("2006-01-02")
	fits, observed, err := observedScheduleFits(day)
	if err != nil {
		log.Printf("calendar: timetable check for %s failed: %v", date, err)
		return
	}
	expected := scheduleDay(day)
	if observed < observedMinRuns {
		log.Printf("calendar: %s had %d runs, too few to tell which timetable ran", date, observed)
		return
	}
	best, current := fits[0], scheduleFit{}
	for _, f := range fits {
		if f.Day == expected {
			current = f
		}
	}
	log.Printf("calendar: %s fits %s", date, describeFits(fits, observed))
	if best.Day == expected || best.Fit < observedMinFit || best.Fit-current.Fit < observedMinLead {
		return
	}
	if existing, ok, err := database.ScheduleOverrideOn(date); err == nil && ok && existing.Source == overrideByAdmin {
		log.Printf("calendar: %s looked like the %s timetable, but it was set to %s on the admin page; keeping that", date, best.Day, existing.Schedule)
		return
	}
	err = database.SaveScheduleOverride(database.ScheduleOverride{
		Date: date, Schedule: best.Day, Source: overrideObserved,
		Detail:    fmt.Sprintf("expected %s; %s", expected, describeFits(fits, observed)),
		CreatedAt: now.Unix(),
	})
	if err != nil {
		log.Printf("calendar: saving the %s override for %s failed: %v", best.Day, date, err)
		return
	}
	log.Printf("calendar: %s ran the %s timetable, not %s; relearning", date, best.Day, expected)
	if err := LoadCalendar(); err != nil {
		log.Printf("calendar: reload failed: %v", err)
		return
	}
	applyCalendar(now)
}

func neededSchoolYear(now time.Time) int {
	local := now.In(database.StatsLocation())
	if local.Month() >= time.August {
		return local.Year()
	}
	return local.Year() - 1
}

func calendarProblem(now time.Time) string {
	if y := neededSchoolYear(now); !currentCalendar().HasYear(y) {
		return fmt.Sprintf("Enter the %d–%d school year in the calendar", y, y+1)
	}
	return ""
}

type adminCalendarYear struct {
	calendar.SchoolYear
	Status string `json:"status"`
}

type adminHoliday struct {
	Date     string `json:"date"`
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
}

type adminCalendarToday struct {
	Date        string `json:"date"`
	ProfileDay  string `json:"profile_day"`
	ScheduleDay string `json:"schedule_day"`
	Holiday     string `json:"holiday,omitempty"`
}

type adminCalendarView struct {
	Today     adminCalendarToday          `json:"today"`
	Editable  bool                        `json:"editable"`
	Needed    int                         `json:"needed,omitempty"`
	Years     []adminCalendarYear         `json:"years"`
	Holidays  []adminHoliday              `json:"holidays"`
	Overrides []database.ScheduleOverride `json:"overrides"`
	Learning  adminLearning               `json:"learning"`
}

type adminWindow struct {
	DayType      string      `json:"day_type"`
	Days         int         `json:"days"`
	From         string      `json:"from,omitempty"`
	Seed         string      `json:"seed"`
	LastYearDays int         `json:"last_year_days"`
	LastYearFrom string      `json:"last_year_from,omitempty"`
	LastYearTo   string      `json:"last_year_to,omitempty"`
	Evaluation   *seedTuning `json:"evaluation,omitempty"`
}

type adminLearning struct {
	Windows    []adminWindow `json:"windows"`
	Tuning     *windowTuning `json:"tuning,omitempty"`
	Candidates []int         `json:"candidates"`
	MinDays    int           `json:"min_days"`
}

func buildAdminCalendar(now time.Time) (adminCalendarView, error) {
	cal := currentCalendar()
	today := localMidnight(now)
	v := adminCalendarView{
		Today: adminCalendarToday{
			Date: today.Format("2006-01-02"), ProfileDay: cal.ProfileDay(today), ScheduleDay: cal.ScheduleDay(today),
		},
		Editable:  database.Archive != nil,
		Years:     []adminCalendarYear{},
		Holidays:  []adminHoliday{},
		Overrides: []database.ScheduleOverride{},
	}
	if h, ok := cal.Holiday(today); ok {
		v.Today.Holiday = h.Name
	}
	needed := neededSchoolYear(now)
	if !cal.HasYear(needed) {
		v.Needed = needed
	}

	saved, err := savedSchoolYears()
	if err != nil {
		return v, err
	}
	var latest *calendar.SchoolYear
	for _, y := range cal.Years() {
		if latest == nil || y.Year > latest.Year {
			copied := y
			latest = &copied
		}
		if y.Year < needed {
			continue
		}
		status := "built_in"
		if _, ok := saved[y.Year]; ok {
			status = "saved"
		}
		v.Years = append(v.Years, adminCalendarYear{SchoolYear: y.Normalized(), Status: status})
	}
	if latest != nil {
		next := calendar.Estimate(*latest)
		for next.Year < needed {
			next = calendar.Estimate(next)
		}
		v.Years = append(v.Years, adminCalendarYear{SchoolYear: next, Status: "estimate"})
	}

	last := today.AddDate(0, 0, adminHolidayDays)
	for year := today.Year(); year <= last.Year(); year++ {
		for _, h := range calendar.PublicHolidays(year) {
			if h.Date < v.Today.Date || h.Date > last.Format("2006-01-02") {
				continue
			}
			at, _ := time.ParseInLocation("2006-01-02", h.Date, database.StatsLocation())
			v.Holidays = append(v.Holidays, adminHoliday{Date: h.Date, Name: h.Name, Schedule: cal.ScheduleDay(at)})
		}
	}

	overrides, err := database.ScheduleOverrides()
	if err != nil {
		return v, err
	}
	since := today.AddDate(0, 0, -adminOverrideDays).Format("2006-01-02")
	for _, o := range overrides {
		if o.Date >= since {
			v.Overrides = append(v.Overrides, o)
		}
	}

	v.Learning = adminLearning{Windows: []adminWindow{}, Candidates: vacationWindowCandidates, MinDays: tuningMinDays}
	dateOf := func(at int64) string { return time.Unix(at, 0).In(database.StatsLocation()).Format("2006-01-02") }
	for _, day := range calendar.ProfileDays {
		n, _ := dayWindowSize(day)
		w := currentDayWindow(day, now)
		a := adminWindow{DayType: day, Days: n, Seed: seedFill(day), LastYearDays: len(w.lastYear)}
		if since, ok := w.since(); ok {
			a.From = dateOf(since)
		}
		if len(w.lastYear) > 0 {
			a.LastYearFrom = w.lastYear[0].Format("2006-01-02")
			a.LastYearTo = w.lastYear[len(w.lastYear)-1].Format("2006-01-02")
		}
		if t, ok, err := savedSeedTuning(day); err != nil {
			return v, err
		} else if ok {
			a.Evaluation = &t
		}
		v.Learning.Windows = append(v.Learning.Windows, a)
	}
	if t, ok, err := savedWindowTuning(); err != nil {
		return v, err
	} else if ok {
		v.Learning.Tuning = &t
	}
	return v, nil
}

func registerAdminCalendarRoutes(admin fiber.Router, token string) {
	admin.Get("/calendar", RequireAdminSession(token), func(c fiber.Ctx) error {
		view, err := buildAdminCalendar(time.Now())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(view)
	})

	admin.Put("/calendar", RequireAdminSession(token), func(c fiber.Ctx) error {
		if !sameOriginRequest(c) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		var year calendar.SchoolYear
		if err := c.Bind().Body(&year); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		year = year.Normalized()
		if err := year.Validate(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		data, err := json.Marshal(year)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if err := database.SaveSchoolYear(year.Year, string(data), time.Now()); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
		}
		if err := LoadCalendar(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		invalidateScheduleCaches()
		log.Printf("calendar: school year %d–%d saved from the admin page", year.Year, year.Year+1)
		relearnAfterCalendarChange(time.Now())
		view, err := buildAdminCalendar(time.Now())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(view)
	})

	admin.Get("/calendar/day", RequireAdminSession(token), func(c fiber.Ctx) error {
		day, err := parseAdminDay(c.Query("date"), time.Now())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		d, err := buildAdminCalendarDay(day)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(d)
	})

	admin.Put("/calendar/day", RequireAdminSession(token), func(c fiber.Ctx) error {
		if !sameOriginRequest(c) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		var req adminDayRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		now := time.Now()
		day, err := parseAdminDay(req.Date, now)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		date := day.Format("2006-01-02")
		switch req.Schedule {
		case "":
			err = database.DeleteScheduleOverride(date)
		case calendar.Weekday, calendar.Saturday, calendar.Sunday:
			err = database.SaveScheduleOverride(database.ScheduleOverride{
				Date: date, Schedule: req.Schedule, Source: overrideByAdmin, Detail: "set on the admin page", CreatedAt: now.Unix(),
			})
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("unknown timetable %q", req.Schedule)})
		}
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
		}
		if err := LoadCalendar(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		invalidateScheduleCaches()
		log.Printf("calendar: %s set to %s on the admin page", date, cmp.Or(req.Schedule, "automatic"))
		if !day.After(localMidnight(now)) {
			relearnAfterCalendarChange(now)
		}
		view, err := buildAdminCalendar(now)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		d, err := buildAdminCalendarDay(day)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(adminDayResponse{Calendar: view, Day: d})
	})
}

type adminCalendarDay struct {
	Date        string                     `json:"date"`
	ScheduleDay string                     `json:"schedule_day"`
	Automatic   string                     `json:"automatic"`
	ProfileDay  string                     `json:"profile_day"`
	Holiday     string                     `json:"holiday,omitempty"`
	Override    *database.ScheduleOverride `json:"override,omitempty"`
}

type adminDayRequest struct {
	Date     string `json:"date"`
	Schedule string `json:"schedule"`
}

type adminDayResponse struct {
	Calendar adminCalendarView `json:"calendar"`
	Day      adminCalendarDay  `json:"day"`
}

func parseAdminDay(date string, now time.Time) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, database.StatsLocation())
	if err != nil {
		return day, fmt.Errorf("%q is not a date", date)
	}
	today := localMidnight(now)
	if day.Before(today.AddDate(0, 0, -adminDayReach)) || day.After(today.AddDate(0, 0, adminDayReach)) {
		return day, fmt.Errorf("pick a date within %d days of today", adminDayReach)
	}
	return day, nil
}

func buildAdminCalendarDay(day time.Time) (adminCalendarDay, error) {
	cal := currentCalendar()
	d := adminCalendarDay{
		Date:        day.Format("2006-01-02"),
		ScheduleDay: cal.ScheduleDay(day),
		Automatic:   cal.AutomaticScheduleDay(day),
		ProfileDay:  cal.ProfileDay(day),
	}
	if h, ok := cal.Holiday(day); ok {
		d.Holiday = h.Name
	}
	o, ok, err := database.ScheduleOverrideOn(d.Date)
	if err != nil {
		return d, err
	}
	if ok {
		d.Override = &o
	}
	return d, nil
}

func networkScheduleDays(now time.Time) (map[string]string, map[string]models.DayNote) {
	cal := currentCalendar()
	sources := map[string]string{}
	if overrides, err := database.ScheduleOverrides(); err == nil {
		for _, o := range overrides {
			sources[o.Date] = o.Source
		}
	}
	local := now.In(database.StatsLocation())
	days, notes := map[string]string{}, map[string]models.DayNote{}
	end := time.Date(local.Year()+2, time.January, 1, 12, 0, 0, 0, local.Location())
	for d := time.Date(local.Year(), time.January, 1, 12, 0, 0, 0, local.Location()); d.Before(end); d = d.AddDate(0, 0, 1) {
		plain := calendar.Weekday
		switch d.Weekday() {
		case time.Saturday:
			plain = calendar.Saturday
		case time.Sunday:
			plain = calendar.Sunday
		}
		s := cal.ScheduleDay(d)
		if s == plain {
			continue
		}
		date := d.Format("2006-01-02")
		days[date] = s
		var note models.DayNote
		if h, ok := cal.Holiday(d); ok {
			note = models.DayNote{Reason: dayNoteHoliday, Holiday: h.Key}
		}
		switch sources[date] {
		case overrideByAdmin:
			note.Reason = dayNoteSet
		case overrideObserved:
			note.Reason = dayNoteObserved
		}
		notes[date] = note
	}
	return days, notes
}
