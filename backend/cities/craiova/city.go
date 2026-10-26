// Package craiova is Craiova: RAT Craiova's network and buses from its Modeshift journey planner.
package craiova

import "conexiuni-cluj/cities"

const modeshift = "https://app.craiova-transport.com/api/v1/ba79f1ee-c6f7-48ad-8a42-d23417a3ab53"

var City = cities.Spec{
	Slug: "craiova", Name: "Craiova", Operator: "RAT Craiova", OperatorURL: "https://ratcraiova.ro/", County: "dolj",
	// No news: ratcraiova.ro has no feed.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return cities.Feeds{
			Static:   env.GTFS(modeshift + "/transport/gtfs/gtfs.zip"),
			Realtime: env.GTFSRT(modeshift + "/transport/gtfsrt/tripUpdates.pb"),
		}, nil
	},
}
