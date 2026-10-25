// Package galati is Galați: Transurb's network from a volunteer-made GTFS, timetables only.
package galati

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/feeds"
)

const volunteer = "https://codeberg.org/rapiteanu/public-transport"

var City = cities.Spec{
	Slug: "galati", Name: "Galați", Operator: "Transurb Galați", OperatorURL: "https://transurbgalati.ro/",
	// No news: transurbgalati.ro's announcements have no titles or links.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static: feeds.WithCredit(env.GTFS(volunteer+"/releases/download/gtfs-transurb/gtfs-transurb.zip"),
				feeds.Credit{Name: "Viorel-Cătălin Răpițeanu (CC BY 4.0)", URL: volunteer}),
		}, nil
	},
}
