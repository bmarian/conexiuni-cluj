// Package cluj is Cluj-Napoca: CTP's network and buses from Tranzy, its timetables from CTP's own CSVs.
package cluj

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "cluj", Name: "Cluj-Napoca", Operator: "CTP Cluj-Napoca", OperatorURL: "https://ctpcj.ro/", County: "cluj",
	// The city ticket does not cover the metropolitan (M) lines.
	// The hidden stops are the Disp. stands, where buses wait between runs.
	Rules: cities.Rules{OwnTicket: cities.LinesStartingWith("M"), HiddenStops: []int{1, 35, 46, 143, 154, 182, 223, 246, 254}},
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Tranzy(2)
		f.Timetables = newTimetables(env.Config.CtpCsvBaseUrl, env.Config.CtpCjRateLimit, env.Config.TimetableRefresh)
		f.News = env.RSS("https://www.ctpcj.ro/index.php/ro/despre-noi/stiri?format=feed&type=rss")
		return f, err
	},
}
