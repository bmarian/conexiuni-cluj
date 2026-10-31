// Package cities runs the server's cities: a Spec, filled in by each city in its own folder, says what a city reads, and a City wires its database, live side and services together.
package cities

import (
	"conexiuni-cluj/feeds"
	"errors"
	"strings"
)

type Spec struct {
	// The URL path and the database file name.
	Slug        string
	Name        string
	Operator    string
	OperatorURL string
	// The county's slug on calendarscolar.ro.
	County string
	Rules  Rules
	Feeds  func(env *Env) (Feeds, error)
}

type Rules struct {
	// Lines the city ticket does not cover; nil when it covers them all.
	OwnTicket func(routeShortName string) bool
	HiddenStops []int
}

func LinesStartingWith(prefix string) func(string) bool {
	return func(name string) bool { return strings.HasPrefix(name, prefix) }
}

type Feeds struct {
	Static feeds.Static
	// Departure times laid over a Static that has none of its own.
	Timetables feeds.Timetables
	// Nil for a city with timetables only.
	Realtime feeds.Realtime
	// Polled while nobody watches; Realtime when nil.
	Learning feeds.Realtime
	News     feeds.News
}

// A city whose key is missing is left out, not failed.
var ErrNoKey = errors.New("no API key")
