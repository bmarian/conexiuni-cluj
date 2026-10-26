// Package cluj is Cluj-Napoca: CTP's network and buses from Tranzy, its timetables from CTP's own CSVs.
package cluj

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "cluj", Name: "Cluj-Napoca", Operator: "CTP Cluj-Napoca", OperatorURL: "https://ctpcj.ro/", County: "cluj",
	// The city ticket does not cover the metropolitan (M) lines.
	Rules: cities.Rules{OwnTicket: cities.LinesStartingWith("M")},
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Tranzy(2)
		f.Timetables = newTimetables(env.Config.CtpCsvBaseUrl, env.Config.CtpCjRateLimit, env.Config.TimetableRefresh)
		f.News = env.RSS("https://www.ctpcj.ro/index.php/ro/despre-noi/stiri?format=feed&type=rss")
		return f, err
	},
}
