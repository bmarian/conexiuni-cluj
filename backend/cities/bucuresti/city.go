// Package bucuresti is București: TPBI's regional GTFS and GTFS-realtime.
package bucuresti

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "bucuresti", Name: "București", Operator: "TPBI", OperatorURL: "https://tpbi.ro/",
	// The metro (M1 to M5) takes its own ticket.
	Rules: cities.Rules{OwnTicket: cities.LinesStartingWith("M")},
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static:   env.GTFS("https://gtfs.tpbi.ro/regional/BUCHAREST-REGION.zip"),
			Realtime: env.GTFSRT("https://gtfs.tpbi.ro/api/gtfs-rt/vehiclePositions"),
			News:     env.RSS("https://tpbi.ro/feed/"),
		}, nil
	},
}
