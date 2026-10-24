// Package ploiesti is Ploiești: TCE's network from a volunteer-made GTFS zip and its buses from gtfs.ro's GTFS-realtime feed.
package ploiesti

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "ploiesti", Name: "Ploiești", Operator: "TCE Ploiești", OperatorURL: "https://tceph.ro/",
	// The live feed runs ahead of the published zip: buses of the lines it lacks (TCE1D, TCE30A, TCE32b on 2026-10-24) are left out.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static:   env.GTFS("https://gtfs.opendev.ro/GTFS-305.zip"),
			Realtime: env.GTFSRT("https://ploiesti-rt-feed.gtfs.ro/vehiclePositions"),
			News:     env.RSS("https://tceph.ro/feed/"),
		}, nil
	},
}
