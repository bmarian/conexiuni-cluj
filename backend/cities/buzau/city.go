// Package buzau is Buzău: Trans Bus's network from a volunteer-made GTFS zip and its buses from a GTFS-realtime feed.
package buzau

import "conexiuni-cluj/cities"

var City = cities.Spec{
	// transbusbuzau.ro carries injected spam.
	Slug: "buzau", Name: "Buzău", Operator: "Trans Bus Buzău", OperatorURL: "https://primariabuzau.ro/", County: "buzau",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static:   env.GTFS("https://gtfs.opendev.ro/GTFS-301.zip"),
			Realtime: env.GTFSRT("https://gtfsagent.transportpublic.ro/transbus/vehiclePositions"),
		}, nil
	},
}
