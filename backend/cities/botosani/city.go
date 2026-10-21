// Package botosani is Botoșani: Eltrans's network from Tranzy, its headways from the town hall's PDF, timetables only.
package botosani

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "botosani", Name: "Botoșani", Operator: "Eltrans Botoșani", OperatorURL: "https://eltransbt.ro/",
	// No news: eltransbt.ro posts only job ads.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Tranzy(6)
		f.Timetables = newTimetables("https://primariabt.ro/storage/2020/07/microbusautobus.pdf", env.Config.TimetableRefresh)
		// Too few of Tranzy's buses report to show live; they still feed the learner.
		if f.Learning == nil {
			f.Learning = f.Realtime
		}
		f.Realtime = nil
		return f, err
	},
}
