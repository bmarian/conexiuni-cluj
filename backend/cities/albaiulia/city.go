// Package albaiulia is Alba Iulia from STP's own timetable pages, timetables only.
package albaiulia

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/feeds/gtfs"
	"strings"
)

var City = cities.Spec{
	Slug: "albaiulia", Name: "Alba Iulia", Operator: "STP Alba Iulia", OperatorURL: "https://www.stpalba.ro/", County: "alba",
	// STP lists each line under the fare zone of its first digit; the city ticket covers zone 1 only.
	Rules: cities.Rules{OwnTicket: func(line string) bool { return !strings.HasPrefix(line, "1") }},
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		if _, err := env.Store(); err != nil {
			return cities.Feeds{}, err
		}
		pages := &site{index: "https://www.stpalba.ro/program.php", school: env.School()}
		return cities.Feeds{
			Static: gtfs.Built("stpalba.ro", pages.zip, env.Config.TimetableRefresh, env.IDs()),
			News:   env.News("https://www.stpalba.ro/presa.php", parseNews),
		}, nil
	},
}
