// Package brasov is Brașov: RATBV's network from the volunteer osm2gtfs zip, made from OpenStreetMap and RATBV's timetables; timetables only.
package brasov

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/feeds"
	"strconv"
)

const osm2gtfs = "https://github.com/szjozsef/osm2gtfs"

var City = cities.Spec{
	Slug: "brasov", Name: "Brașov", Operator: "RATBV", OperatorURL: "https://www.ratbv.ro/", County: "brasov",
	Rules: cities.Rules{OwnTicket: metropolitan},
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static: feeds.WithCredit(env.GTFS("https://raw.githubusercontent.com/szjozsef/osm2gtfs/refs/heads/master/output/gtfs/ro-ratbv.zip"),
				feeds.Credit{Name: "osm2gtfs, OpenStreetMap contributors (ODbL)", URL: osm2gtfs}),
			News: env.News("https://www.ratbv.ro/noutati/", parseNews),
		}, nil
	},
}

// RATBV's metropolitan lines, which the urban ticket does not cover, are numbered past 100; 100 itself is urban.
func metropolitan(line string) bool {
	n, err := strconv.Atoi(line)
	return err == nil && n > 100
}
