// Package iasi is Iași: CTP Iași's network and buses from Tranzy, its timetables from gtfs.ro.
package iasi

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "iasi", Name: "Iași", Operator: "CTP Iași", OperatorURL: "https://www.sctpiasi.ro/",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Tranzy(1)
		f.Timetables = env.GTFSTimetables(cities.GTFSRo("iasi"))
		f.News = env.News("https://www.sctpiasi.ro/ro/stiri", parseNews)
		return f, err
	},
}
