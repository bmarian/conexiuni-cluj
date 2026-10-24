// Package targumures is Târgu Mureș: Transport Local's GTFS zip and the buses on its live map.
package targumures

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "targumures", Name: "Târgu Mureș", Operator: "Transport Local", OperatorURL: "https://transportlocal.ro/",
	// No news: transportlocal.ro has no feed.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		if _, err := env.Store(); err != nil {
			return cities.Feeds{}, err
		}
		return cities.Feeds{
			Static:   env.GTFS("https://live.transportlocal.ro/gtfs.zip"),
			Realtime: newRealtime("https://live.transportlocal.ro/journeys.json", env.IDs(), env.Tracing()),
		}, nil
	},
}
