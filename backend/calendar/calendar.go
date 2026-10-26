package calendar

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	Weekday         = "weekday"
	WeekdayVacation = "weekday_vacation"
	Saturday        = "saturday"
	Sunday          = "sunday"

	dateLayout   = "2006-01-02"
	maxBreaks    = 12
	MaxBreakName = 40
)

var ProfileDays = []string{Weekday, WeekdayVacation, Saturday, Sunday}

var ScheduleDays = []string{Weekday, Saturday, Sunday}

type Range struct {
	Name  string `json:"name"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type SchoolYear struct {
	Year   int     `json:"year"`
	Start  string  `json:"start"`
	End    string  `json:"end"`
	Breaks []Range `json:"breaks"`
}

type Holiday struct {
	Date string `json:"date"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type Calendar struct {
	years     []SchoolYear
	overrides map[string]string

	mu       sync.Mutex
	holidays map[int]map[string]Holiday
}

func New(years []SchoolYear, overrides map[string]string) *Calendar {
	sorted := append([]SchoolYear(nil), years...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Year < sorted[j].Year })
	o := make(map[string]string, len(overrides))
	for date, day := range overrides {
		o[date] = day
	}
	return &Calendar{years: sorted, overrides: o, holidays: make(map[int]map[string]Holiday)}
}

func (c *Calendar) Years() []SchoolYear {
	return append([]SchoolYear(nil), c.years...)
}

func (c *Calendar) ProfileDay(t time.Time) string {
	date := t.Format(dateLayout)
	switch c.overrides[date] {
	case Saturday:
		return Saturday
	case Sunday:
		return Sunday
	case Weekday:
		return c.schoolDay(date)
	}
	return c.automaticProfileDay(t, date)
}

func (c *Calendar) AutomaticScheduleDay(t time.Time) string {
	return ScheduleOf(c.automaticProfileDay(t, t.Format(dateLayout)))
}

func (c *Calendar) automaticProfileDay(t time.Time, date string) string {
	if _, ok := c.Holiday(t); ok {
		return Sunday
	}
	switch t.Weekday() {
	case time.Saturday:
		return Saturday
	case time.Sunday:
		return Sunday
	}
	return c.schoolDay(date)
}

func (c *Calendar) ScheduleDay(t time.Time) string {
	return ScheduleOf(c.ProfileDay(t))
}

func ScheduleOf(profileDay string) string {
	if profileDay == WeekdayVacation {
		return Weekday
	}
	return profileDay
}

func Siblings(a, b string) bool {
	return a != b && ScheduleOf(a) == ScheduleOf(b)
}

func (c *Calendar) schoolDay(date string) string {
	covered := false
	for _, y := range c.years {
		if date < y.coverStart() || date > y.coverEnd() {
			continue
		}
		covered = true
		if date >= y.Start && date <= y.End && !y.inBreak(date) {
			return Weekday
		}
	}
	if covered {
		return WeekdayVacation
	}
	return Weekday
}

func (c *Calendar) Covers(t time.Time) bool {
	date := t.Format(dateLayout)
	for _, y := range c.years {
		if date >= y.coverStart() && date <= y.coverEnd() {
			return true
		}
	}
	return false
}

func (c *Calendar) HasYear(year int) bool {
	for _, y := range c.years {
		if y.Year == year {
			return true
		}
	}
	return false
}

func (y SchoolYear) coverStart() string { return fmt.Sprintf("%04d-07-01", y.Year) }
func (y SchoolYear) coverEnd() string   { return fmt.Sprintf("%04d-08-31", y.Year+1) }

func (y SchoolYear) inBreak(date string) bool {
	for _, b := range y.Breaks {
		if date >= b.Start && date <= b.End {
			return true
		}
	}
	return false
}

func (c *Calendar) Holiday(t time.Time) (Holiday, bool) {
	c.mu.Lock()
	byDate, ok := c.holidays[t.Year()]
	if !ok {
		byDate = make(map[string]Holiday)
		for _, h := range PublicHolidays(t.Year()) {
			byDate[h.Date] = h
		}
		c.holidays[t.Year()] = byDate
	}
	c.mu.Unlock()
	h, ok := byDate[t.Format(dateLayout)]
	return h, ok
}

func OrthodoxEaster(year int) time.Time {
	a, b, c := year%4, year%7, year%19
	d := (19*c + 15) % 30
	e := (2*a + 4*b - d + 34) % 7
	month := (d + e + 114) / 31
	day := (d+e+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 13)
}

func PublicHolidays(year int) []Holiday {
	fixed := func(month time.Month, day int, key, name string) Holiday {
		return Holiday{Date: time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(dateLayout), Name: name, Key: key}
	}
	easter := OrthodoxEaster(year)
	around := func(days int, key, name string) Holiday {
		return Holiday{Date: easter.AddDate(0, 0, days).Format(dateLayout), Name: name, Key: key}
	}
	out := []Holiday{
		fixed(time.January, 1, "new_year", "New Year's Day"),
		fixed(time.January, 2, "new_year_2", "New Year holiday"),
		fixed(time.January, 6, "epiphany", "Epiphany"),
		fixed(time.January, 7, "saint_john", "Saint John"),
		fixed(time.January, 24, "union_day", "Union Day"),
		around(-2, "good_friday", "Good Friday"),
		around(0, "easter", "Easter Sunday"),
		around(1, "easter_monday", "Easter Monday"),
		fixed(time.May, 1, "labour_day", "Labour Day"),
		fixed(time.June, 1, "childrens_day", "Children's Day"),
		around(49, "pentecost", "Pentecost Sunday"),
		around(50, "pentecost_monday", "Pentecost Monday"),
		fixed(time.August, 15, "assumption", "Assumption"),
		fixed(time.November, 30, "saint_andrew", "Saint Andrew"),
		fixed(time.December, 1, "national_day", "National Day"),
		fixed(time.December, 25, "christmas", "Christmas Day"),
		fixed(time.December, 26, "christmas_2", "Christmas holiday"),
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func Seed() []SchoolYear {
	return []SchoolYear{{
		Year:  2026,
		Start: "2026-09-07",
		End:   "2027-06-18",
		Breaks: []Range{
			{Name: "Autumn", Start: "2026-10-24", End: "2026-11-01"},
			{Name: "Winter", Start: "2026-12-23", End: "2027-01-10"},
			{Name: "Ski week", Start: "2027-02-15", End: "2027-02-21"},
			{Name: "Easter", Start: "2027-04-24", End: "2027-05-04"},
		},
	}}
}

func Estimate(prev SchoolYear) SchoolYear {
	shift := func(date string) string {
		t, err := time.Parse(dateLayout, date)
		if err != nil {
			return date
		}
		return t.AddDate(0, 0, 364).Format(dateLayout)
	}
	next := SchoolYear{Year: prev.Year + 1, Start: shift(prev.Start), End: shift(prev.End)}
	oldEaster, newEaster := OrthodoxEaster(prev.Year+1), OrthodoxEaster(prev.Year+2)
	for _, b := range prev.Breaks {
		start, errStart := time.Parse(dateLayout, b.Start)
		end, errEnd := time.Parse(dateLayout, b.End)
		if errStart != nil || errEnd != nil {
			continue
		}
		if !start.After(oldEaster.AddDate(0, 0, 21)) && !end.Before(oldEaster.AddDate(0, 0, -21)) {
			next.Breaks = append(next.Breaks, Range{
				Name:  b.Name,
				Start: newEaster.Add(start.Sub(oldEaster)).Format(dateLayout),
				End:   newEaster.Add(end.Sub(oldEaster)).Format(dateLayout),
			})
			continue
		}
		next.Breaks = append(next.Breaks, Range{Name: b.Name, Start: shift(b.Start), End: shift(b.End)})
	}
	return next
}

func (y SchoolYear) Validate() error {
	if y.Year < 2020 || y.Year > 2100 {
		return fmt.Errorf("year %d is out of range", y.Year)
	}
	start, err := time.Parse(dateLayout, y.Start)
	if err != nil {
		return fmt.Errorf("term start %q is not a date", y.Start)
	}
	end, err := time.Parse(dateLayout, y.End)
	if err != nil {
		return fmt.Errorf("term end %q is not a date", y.End)
	}
	if start.Year() != y.Year || start.Month() < time.August || start.Month() > time.October {
		return fmt.Errorf("term start should fall in August–October %d", y.Year)
	}
	if end.Year() != y.Year+1 || end.Month() < time.May || end.Month() > time.July {
		return fmt.Errorf("term end should fall in May–July %d", y.Year+1)
	}
	if len(y.Breaks) > maxBreaks {
		return fmt.Errorf("at most %d breaks", maxBreaks)
	}
	for _, b := range y.Breaks {
		name := strings.TrimSpace(b.Name)
		if name == "" || len(name) > MaxBreakName {
			return fmt.Errorf("every break needs a name of up to %d characters", MaxBreakName)
		}
		bs, err := time.Parse(dateLayout, b.Start)
		if err != nil {
			return fmt.Errorf("%s: start %q is not a date", name, b.Start)
		}
		be, err := time.Parse(dateLayout, b.End)
		if err != nil {
			return fmt.Errorf("%s: end %q is not a date", name, b.End)
		}
		if be.Before(bs) {
			return fmt.Errorf("%s ends before it starts", name)
		}
		if bs.Before(start) || be.After(end) {
			return fmt.Errorf("%s falls outside the term", name)
		}
	}
	return nil
}

func (y SchoolYear) Normalized() SchoolYear {
	out := SchoolYear{Year: y.Year, Start: y.Start, End: y.End, Breaks: make([]Range, 0, len(y.Breaks))}
	for _, b := range y.Breaks {
		out.Breaks = append(out.Breaks, Range{Name: strings.TrimSpace(b.Name), Start: b.Start, End: b.End})
	}
	sort.SliceStable(out.Breaks, func(i, j int) bool { return out.Breaks[i].Start < out.Breaks[j].Start })
	return out
}

func (y SchoolYear) Equal(o SchoolYear) bool {
	a, b := y.Normalized(), o.Normalized()
	return a.Year == b.Year && a.Start == b.Start && a.End == b.End && slices.Equal(a.Breaks, b.Breaks)
}
