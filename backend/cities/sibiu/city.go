// Package sibiu is Sibiu: Tursib's GTFS, timetables only.
package sibiu

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "sibiu", Name: "Sibiu", Operator: "Tursib", OperatorURL: "https://tursib.ro/",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static: env.GTFS("https://tursib.ro/trasee/gtfs"),
			News:   env.News("https://tursib.ro/news", parseNews),
		}, nil
	},
}
