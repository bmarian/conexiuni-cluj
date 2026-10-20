// Package timisoara is Timișoara: SMTT's GTFS, the vehicles from the live.stpt.ro map.
package timisoara

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "timisoara", Name: "Timișoara", Operator: "STPT Timișoara", OperatorURL: "https://stpt.ro/",
	// The metropolitan (M) lines take their own ticket.
	Rules: cities.Rules{OwnTicket: cities.LinesStartingWith("M")},
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static:   env.GTFS("https://smtt.ro/transit/gtfs/maps/smtt-timisoara-ro.zip"),
			Realtime: newRealtime("https://live.stpt.ro/gtfs-vehicles.php", env.IDs()),
			News:     env.RSS("https://stpt.ro/feed/"),
		}, nil
	},
}
