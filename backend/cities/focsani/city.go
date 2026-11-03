// Package focsani is Focșani: Transport Public Focșani's network and buses from its Modeshift journey planner.
package focsani

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "focsani", Name: "Focșani", Operator: "Transport Public Focșani", OperatorURL: "https://www.tpfocsani.ro/", County: "vrancea",
	// No news: tpfocsani.ro has no feed.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/6eeb6057-bef2-458a-aab0-fd6524d81a4a")
	},
}
