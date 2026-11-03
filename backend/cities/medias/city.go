// Package medias is Mediaș: Meditur's network and buses from its Modeshift journey planner.
package medias

import "conexiuni-cluj/cities"

var City = cities.Spec{
	// meditur.ro carries injected spam, so the operator links its journey planner; no news for the same reason.
	Slug: "medias", Name: "Mediaș", Operator: "Meditur", OperatorURL: "https://medias-transport.telelink.city/", County: "sibiu",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/86f02d07-300e-4c92-a71e-96c9561cc87a")
	},
}
