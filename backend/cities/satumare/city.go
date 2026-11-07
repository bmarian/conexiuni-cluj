// Package satumare is Satu Mare: Transurban's network from the GTFS zip gtfs.ro keeps, with the departures of tusm.ro's line pages; timetables only.
package satumare

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "satumare", Name: "Satu Mare", Operator: "Transurban Satu Mare", OperatorURL: "https://www.tusm.ro/", County: "satu-mare",
	// No news: tusm.ro's feed address answers with a page.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		zip := env.GTFS(cities.GTFSRo("satumare"))
		return cities.Feeds{Static: &static{zip: zip, times: timetables{index: "https://www.tusm.ro/liniistatiiorare"}}}, nil
	},
}
