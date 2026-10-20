// Package constanta is Constanța: CT BUS's network and buses from Tranzy, its timetables from gtfs.ro.
package constanta

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "constanta", Name: "Constanța", Operator: "CT BUS Constanța", OperatorURL: "https://www.ctbus.ro/",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Tranzy(10)
		f.Timetables = env.GTFSTimetables(cities.GTFSRo("constanta"))
		f.News = env.News("https://www.ctbus.ro/comunicate", parseNews)
		return f, err
	},
}
