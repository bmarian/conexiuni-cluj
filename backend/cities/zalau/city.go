// Package zalau is Zalău: Transurbis's network from the GTFS zip gtfs.ro keeps, timetables only.
package zalau

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "zalau", Name: "Zalău", Operator: "Transurbis Zalău", OperatorURL: "https://www.tuz.ro/", County: "salaj",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static: env.GTFS(cities.GTFSRo("zalau")),
			News:   env.RSS("https://www.tuz.ro/feed/"),
		}, nil
	},
}
