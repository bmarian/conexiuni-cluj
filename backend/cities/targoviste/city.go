// Package targoviste is Târgoviște: SPMT's network from a volunteer-made GTFS zip, timetables only.
package targoviste

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "targoviste", Name: "Târgoviște", Operator: "Servicii Publice Municipale Târgoviște", OperatorURL: "http://spmt.ro/", County: "dambovita",
	// No news: spmt.ro has no feed.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{Static: env.GTFS("https://gtfs.opendev.ro/GTFS-306.zip")}, nil
	},
}
