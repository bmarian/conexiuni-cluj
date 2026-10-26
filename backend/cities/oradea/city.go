// Package oradea is Oradea: OTL's network and vehicles from Tranzy, its timetables from gtfs.ro.
package oradea

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "oradea", Name: "Oradea", Operator: "OTL Oradea", OperatorURL: "https://www.otl.ro/", County: "bihor",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Tranzy(9)
		f.Timetables = env.GTFSTimetables(cities.GTFSRo("oradea"))
		f.News = env.RSS("https://www.otl.ro/index.php/ro/?format=feed&type=rss")
		return f, err
	},
}
