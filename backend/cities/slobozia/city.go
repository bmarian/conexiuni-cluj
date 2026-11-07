// Package slobozia is Slobozia: SPTL's network from the timetables in sptl.ro's own scripts, and the buses on its live map.
package slobozia

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/feeds/gtfs"
)

var City = cities.Spec{
	Slug: "slobozia", Name: "Slobozia", Operator: "SPTL Slobozia", OperatorURL: "https://www.sptl.ro/", County: "ialomita",
	// No news: sptl.ro has no news page or feed, and sptl-slobozia.ro now leads to it.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		if _, err := env.Store(); err != nil {
			return cities.Feeds{}, err
		}
		pages := &site{home: "https://www.sptl.ro/", school: env.School()}
		return cities.Feeds{
			Static:   gtfs.Built("sptl.ro", pages.zip, env.Config.TimetableRefresh, env.IDs()),
			Realtime: newRealtime("https://www.sptl.ro/api/bus-data?history=0", env.IDs(), env.Tracing()),
		}, nil
	},
}
