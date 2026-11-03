// Package sinaia is Sinaia: Transport Urban Sinaia's network and buses from its Modeshift journey planner.
package sinaia

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "sinaia", Name: "Sinaia", Operator: "Transport Urban Sinaia", OperatorURL: "https://sinaiago.ro/transport-urban/", County: "prahova",
	// No news: sinaiago.ro's feed holds only test posts.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://app.sinaia-transport.com/api/v1/3b8a4c72-d9dc-4f1d-a36f-f7ca798a660c")
	},
}
