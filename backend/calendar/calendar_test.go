package calendar

import (
	"testing"
	"time"
)

func day(date string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", date+" 08:00", time.FixedZone("EET", 2*3600))
	if err != nil {
		panic(err)
	}
	return t
}

func TestOrthodoxEaster(t *testing.T) {
	for year, want := range map[int]string{2024: "2024-05-05", 2025: "2025-04-20", 2026: "2026-04-12", 2027: "2027-05-02", 2028: "2028-04-16"} {
		if got := OrthodoxEaster(year).Format("2006-01-02"); got != want {
			t.Errorf("OrthodoxEaster(%d) = %s, want %s", year, got, want)
		}
	}
}

func TestPublicHolidaysFollowEaster(t *testing.T) {
	names := map[string]string{}
	for _, h := range PublicHolidays(2027) {
		names[h.Date] = h.Name
	}
	for date, want := range map[string]string{
		"2027-04-30": "Good Friday",
		"2027-05-03": "Easter Monday",
		"2027-06-20": "Pentecost Sunday",
		"2027-06-21": "Pentecost Monday",
		"2027-01-07": "Saint John",
	} {
		if names[date] != want {
			t.Errorf("%s = %q, want %q", date, names[date], want)
		}
	}
	if len(names) != 17 {
		t.Errorf("%d holidays, want 17", len(names))
	}
}

func TestProfileDay(t *testing.T) {
	c := New(Seed(), map[string]string{"2026-12-24": Saturday})
	cases := map[string]string{
		"2026-09-03": WeekdayVacation,
		"2026-09-05": Saturday,
		"2026-09-07": Weekday,
		"2026-10-23": Weekday,
		"2026-10-26": WeekdayVacation,
		"2026-11-02": Weekday,
		"2026-11-30": Sunday,
		"2026-12-01": Sunday,
		"2026-12-24": Saturday,
		"2027-02-16": WeekdayVacation,
		"2027-06-18": Weekday,
		"2027-06-21": Sunday,
		"2027-06-22": WeekdayVacation,
		"2027-08-31": WeekdayVacation,
		"2027-09-01": Weekday,
		"2026-06-30": Weekday,
		"2026-07-01": WeekdayVacation,
	}
	for date, want := range cases {
		if got := c.ProfileDay(day(date)); got != want {
			t.Errorf("ProfileDay(%s) = %s, want %s", date, got, want)
		}
	}
	if got := c.ScheduleDay(day("2026-10-26")); got != Weekday {
		t.Errorf("a school-holiday weekday runs the %s timetable, want weekday", got)
	}
}

func TestWeekdayOverrideStillKnowsTheTerm(t *testing.T) {
	c := New(Seed(), map[string]string{"2026-12-01": Weekday, "2026-10-27": Weekday})
	if got := c.ProfileDay(day("2026-12-01")); got != Weekday {
		t.Errorf("holiday run as a weekday in term = %s", got)
	}
	if got := c.ProfileDay(day("2026-10-27")); got != WeekdayVacation {
		t.Errorf("weekday override in a break = %s", got)
	}
}

func TestEstimateKeepsWeekdaysAndFollowsEaster(t *testing.T) {
	prev := Seed()[0]
	next := Estimate(prev)
	if next.Year != 2027 || next.Start != "2027-09-06" || next.End != "2028-06-16" {
		t.Fatalf("estimate %d %s–%s", next.Year, next.Start, next.End)
	}
	for i, b := range next.Breaks {
		if b.Name == "Easter" {
			easter := OrthodoxEaster(2028)
			if day(b.Start).After(easter) || day(b.End).Before(easter) {
				t.Errorf("Easter break %s–%s misses Easter %s", b.Start, b.End, easter.Format("2006-01-02"))
			}
			continue
		}
		if day(b.Start).Weekday() != day(prev.Breaks[i].Start).Weekday() {
			t.Errorf("%s moved from a %s to a %s", b.Name, day(prev.Breaks[i].Start).Weekday(), day(b.Start).Weekday())
		}
	}
	if err := next.Validate(); err != nil {
		t.Errorf("estimate does not validate: %v", err)
	}
}

func TestValidate(t *testing.T) {
	good := Seed()[0]
	if err := good.Validate(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	bad := map[string]func(*SchoolYear){
		"start in spring":  func(y *SchoolYear) { y.Start = "2026-04-01" },
		"end next autumn":  func(y *SchoolYear) { y.End = "2027-10-01" },
		"break before end": func(y *SchoolYear) { y.Breaks[0].End = "2026-10-01" },
		"break outside":    func(y *SchoolYear) { y.Breaks[0].Start = "2026-08-01" },
		"unnamed break":    func(y *SchoolYear) { y.Breaks[0].Name = "  " },
		"not a date":       func(y *SchoolYear) { y.Breaks[1].End = "soon" },
	}
	for name, mutate := range bad {
		y := Seed()[0]
		mutate(&y)
		if y.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestAutomaticScheduleDayIgnoresOverrides(t *testing.T) {
	c := New(Seed(), map[string]string{"2026-12-01": Weekday, "2026-12-02": Saturday})
	if got := c.ScheduleDay(day("2026-12-01")); got != Weekday {
		t.Errorf("overridden holiday runs %s", got)
	}
	if got := c.AutomaticScheduleDay(day("2026-12-01")); got != Sunday {
		t.Errorf("holiday would automatically run %s, want sunday", got)
	}
	if got := c.AutomaticScheduleDay(day("2026-12-02")); got != Weekday {
		t.Errorf("plain Wednesday would automatically run %s", got)
	}
}
