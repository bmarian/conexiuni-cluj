package sitegtfs

import (
	"conexiuni-cluj/feeds"
	"slices"
	"strings"
	"time"
)

// The days a timetable's footnote keeps a departure to; empty for every day of its kind.
type Limit string

const (
	Term     Limit = "term"
	Holidays Limit = "holidays"
)

var weekdays = map[string]time.Weekday{"luni": time.Monday, "marti": time.Tuesday, "miercuri": time.Wednesday, "joi": time.Thursday, "vineri": time.Friday, "sambata": time.Saturday, "duminica": time.Sunday}

// LimitOf reads a Romanian footnote: school terms, school holidays, or one weekday; "nu circulă în vacanțe" is school terms.
func LimitOf(footnote string) (Limit, bool) {
	text := feeds.NormalizeStopLabel(footnote)
	off := slices.Contains(strings.Fields(text), "nu")
	if holidays := strings.Contains(text, "vacant"); holidays || strings.Contains(text, "scol") {
		if holidays != off {
			return Holidays, true
		}
		return Term, true
	}
	if off {
		return "", false
	}
	for word, day := range weekdays {
		if strings.Contains(text, word) {
			return Only(day), true
		}
	}
	return "", false
}

// Only is one weekday's limit.
func Only(day time.Weekday) Limit { return Limit(day.String()) }

func (l Limit) Allows(day time.Time, school feeds.School) bool {
	switch l {
	case "":
		return true
	case Term:
		return school.SchoolDay(day)
	case Holidays:
		return !school.SchoolDay(day)
	}
	return day.Weekday().String() == string(l)
}
