// Package calendarscolar reads a county's school years from calendarscolar.ro's calendar feed.
package calendarscolar

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	origin = "https://calendarscolar.ro"
	// The Ministry's order and each county's ski week are announced months ahead.
	cadence       = 7 * 24 * time.Hour
	icsDateLayout = "20060102"
)

type County string

func (c County) Page() string           { return origin + "/judet/" + string(c) }
func (c County) Cadence() time.Duration { return cadence }

func (c County) SchoolYears() ([]calendar.SchoolYear, error) {
	data, err := feeds.Download(origin + "/api/calendar/county/" + string(c))
	if err != nil {
		return nil, err
	}
	years := Parse(data)
	if len(years) == 0 {
		return nil, fmt.Errorf("%s: no whole school year in the feed", c)
	}
	return years, nil
}

// A school year is the days between two summer holidays.
func Parse(ics []byte) []calendar.SchoolYear {
	var vacations, daysOff []event
	for _, e := range events(ics) {
		switch {
		case e.vacation():
			vacations = append(vacations, e)
		case e.dayOff():
			daysOff = append(daysOff, e)
		}
	}
	sort.Slice(vacations, func(i, j int) bool { return vacations[i].start.Before(vacations[j].start) })
	var summers []event
	for _, v := range vacations {
		if v.summer() {
			summers = append(summers, v)
		}
	}
	holidays := calendar.New(nil, nil)
	var years []calendar.SchoolYear
	for i := 0; i+1 < len(summers); i++ {
		first, last := summers[i].end, summers[i+1].start.AddDate(0, 0, -1)
		y := calendar.SchoolYear{Year: first.Year(), Start: first.Format(time.DateOnly), End: last.Format(time.DateOnly)}
		within := func(e event) bool { return !e.start.Before(first) && !e.end.AddDate(0, 0, -1).After(last) }
		for _, v := range vacations {
			if within(v) && !v.summer() {
				y.Breaks = append(y.Breaks, v.asBreak())
			}
		}
		for _, d := range daysOff {
			if within(d) && d.closesSchool(vacations, holidays) {
				y.Breaks = append(y.Breaks, d.asBreak())
			}
		}
		years = append(years, y.Normalized())
	}
	return years
}

type event struct {
	summary, description string
	// end is exclusive.
	start, end time.Time
}

func (e event) vacation() bool {
	return strings.HasPrefix(feeds.NormalizeStopLabel(e.summary), "vacanta")
}

func (e event) dayOff() bool {
	text := feeds.NormalizeStopLabel(e.summary + " " + e.description)
	return strings.Contains(text, "zi libera") || strings.Contains(text, "nu se organizeaza cursuri")
}

func (e event) summer() bool {
	midsummer := time.Date(e.start.Year(), time.August, 1, 0, 0, 0, 0, time.UTC)
	return !e.start.After(midsummer) && e.end.After(midsummer)
}

func (e event) closesSchool(vacations []event, holidays *calendar.Calendar) bool {
	for d := e.start; d.Before(e.end); d = d.AddDate(0, 0, 1) {
		if _, holiday := holidays.Holiday(d); holiday || d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		inVacation := false
		for _, v := range vacations {
			inVacation = inVacation || (!d.Before(v.start) && d.Before(v.end))
		}
		if !inVacation {
			return true
		}
	}
	return false
}

func (e event) asBreak() calendar.Range {
	return calendar.Range{Name: breakName(e.summary), Start: e.start.Format(time.DateOnly), End: e.end.AddDate(0, 0, -1).Format(time.DateOnly)}
}

func breakName(summary string) string {
	name := strings.TrimSpace(summary)
	if i := strings.LastIndex(name, " - "); len(name) > calendar.MaxBreakName && i >= 0 {
		name = strings.TrimSpace(name[i+len(" - "):])
	}
	runes := []rune(name)
	for len(string(runes)) > calendar.MaxBreakName {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimSpace(string(runes))
}

var unescaped = strings.NewReplacer(`\\`, `\`, `\,`, ",", `\;`, ";", `\n`, " ", `\N`, " ")

func events(ics []byte) []event {
	text := strings.ReplaceAll(string(ics), "\r\n", "\n")
	text = strings.NewReplacer("\n ", "", "\n\t", "").Replace(text)
	var out []event
	var current *event
	for _, line := range strings.Split(text, "\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name, _, _ = strings.Cut(name, ";")
		if name == "BEGIN" && value == "VEVENT" {
			current = &event{}
			continue
		}
		if current == nil {
			continue
		}
		switch name {
		case "END":
			if value == "VEVENT" && !current.start.IsZero() {
				if !current.end.After(current.start) {
					current.end = current.start.AddDate(0, 0, 1)
				}
				out = append(out, *current)
			}
			current = nil
		case "DTSTART":
			current.start = icsDate(value)
		case "DTEND":
			current.end = icsDate(value)
		case "SUMMARY":
			current.summary = unescaped.Replace(value)
		case "DESCRIPTION":
			current.description = unescaped.Replace(value)
		}
	}
	return out
}

func icsDate(value string) time.Time {
	if len(value) < len(icsDateLayout) {
		return time.Time{}
	}
	t, err := time.Parse(icsDateLayout, value[:len(icsDateLayout)])
	if err != nil {
		return time.Time{}
	}
	return t
}
