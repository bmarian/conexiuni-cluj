// Package botosani is Botoșani from eltransbt.ro and ADIT's map; timetables only.
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
			mapPage: "https://aditmbt.ro/autoritatea-de-transport/harta-interactiva/",
			cadence: env.Config.TimetableRefresh,
			ids:     env.IDs(),
			school:  env.School(),
		}}, nil
	},
}
