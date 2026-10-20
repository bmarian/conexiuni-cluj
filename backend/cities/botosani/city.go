// Package botosani is Botoșani: Eltrans's network and buses from Tranzy, its headways from the town hall's PDF.
package botosani

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "botosani", Name: "Botoșani", Operator: "Eltrans Botoșani", OperatorURL: "https://eltransbt.ro/",
	// No news: eltransbt.ro posts only job ads.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Tranzy(6)
		f.Timetables = newTimetables("https://primariabt.ro/storage/2020/07/microbusautobus.pdf", env.Config.TimetableRefresh)
		return f, err
	},
}
