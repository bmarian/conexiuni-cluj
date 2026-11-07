// Package ramnicuvalcea is Râmnicu Vâlcea from ETA's own timetable pages, timetables only.
package ramnicuvalcea

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/feeds/gtfs"
)

var City = cities.Spec{
	Slug: "ramnicuvalcea", Name: "Râmnicu Vâlcea", Operator: "ETA Râmnicu Vâlcea", OperatorURL: "https://eta-bus.ro/", County: "valcea",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		if _, err := env.Store(); err != nil {
			return cities.Feeds{}, err
		}
		pages := &site{index: "https://eta-bus.ro/trasee", stops: "https://eta-bus.ro/s/map", school: env.School()}
		return cities.Feeds{
			Static: gtfs.Built("eta-bus.ro", pages.zip, env.Config.TimetableRefresh, env.IDs()),
			News:   env.News("https://eta-bus.ro/comunicate", parseNews),
		}, nil
	},
}
