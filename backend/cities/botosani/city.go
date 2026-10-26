// Package botosani is Botoșani from eltransbt.ro, timetables only.
package botosani

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "botosani", Name: "Botoșani", Operator: "Eltrans Botoșani", OperatorURL: "https://eltransbt.ro/", County: "botosani",
	// No news: eltransbt.ro posts only job ads.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		if _, err := env.Store(); err != nil {
			return cities.Feeds{}, err
		}
		return cities.Feeds{Static: &static{
			page:    "https://eltransbt.ro/trasee",
			cadence: env.Config.TimetableRefresh,
			ids:     env.IDs(),
		}}, nil
	},
}
