// Package suceava is Suceava: TPL's network from a volunteer-made GTFS zip, the lines it lacks from tpl-sv.ro, and the buses on TPL's live map.
package suceava

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "suceava", Name: "Suceava", Operator: "TPL Suceava", OperatorURL: "https://tpl-sv.ro/",
	// The metropolitan lines need a ticket of their own outside the city.
	Rules: cities.Rules{OwnTicket: cities.LinesStartingWith("M")},
	// No news: tpl-sv.ro's feed and posts are empty.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		if _, err := env.Store(); err != nil {
			return cities.Feeds{}, err
		}
		live := newRealtime("https://harta.tpl-sv.ro:8888/?ajax=get_bus_data", env.IDs(), env.Tracing())
		live.rec = env.Traces()
		return cities.Feeds{
			Static:   &static{zip: env.GTFS("https://gtfs.opendev.ro/GTFS-328.zip"), pages: linePagesURL, ids: env.IDs(), rec: env.Traces()},
			Realtime: live,
		}, nil
	},
}
