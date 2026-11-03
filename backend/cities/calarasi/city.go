// Package calarasi is Călărași: Ali Trans Com's network and buses from its Modeshift journey planner.
package calarasi

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "calarasi", Name: "Călărași", Operator: "Ali Trans Com", OperatorURL: "https://alitrans.ro/", County: "calarasi",
	// No news: alitrans.ro has no feed.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/be4243ca-f73e-472d-bc64-ee212d65c1fb")
	},
}
