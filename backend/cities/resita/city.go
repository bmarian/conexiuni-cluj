// Package resita is Reșița: TUR's network, tram included, from a volunteer-made GTFS zip, timetables only.
package resita

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "resita", Name: "Reșița", Operator: "Transport Urban Reșița", OperatorURL: "https://www.turesita.ro/", County: "caras-severin",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static: env.GTFS("https://gtfs.opendev.ro/GTFS-300.zip"),
			News:   env.RSS("https://www.turesita.ro/feed/"),
		}, nil
	},
}
