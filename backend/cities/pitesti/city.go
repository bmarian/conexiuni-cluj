// Package pitesti is Pitești from Publitrans 2000's own timetable pages and stop map, timetables only.
package pitesti

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/feeds/gtfs"
)

const base = "https://www.publitrans2000.ro/"

var City = cities.Spec{
	Slug: "pitesti", Name: "Pitești", Operator: "Publitrans 2000", OperatorURL: base, County: "arges",
	// One ticket rides every Publitrans line, the metropolitan ones included.
	// No news: publitrans2000.ro's announcements have no dates or links.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		if _, err := env.Store(); err != nil {
			return cities.Feeds{}, err
		}
		pages := &site{indexes: []string{base + "linii_oras", base + "linii_metropolitane"}, stops: base + "statii", school: env.School()}
		return cities.Feeds{Static: gtfs.Built("publitrans2000.ro", pages.zip, env.Config.TimetableRefresh, env.IDs())}, nil
	},
}
