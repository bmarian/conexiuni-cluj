// Package nasaud is Năsăud: ANI Tour's county lines from its Modeshift journey planner; the town has no lines of its own.
package nasaud

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "nasaud", Name: "Năsăud", Operator: "ANI Tour Năsăud", OperatorURL: "https://www.anitour.ro/transport/intern", County: "bistrita-nasaud",
	// No news: anitour.ro has no feed.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/a5f27685-3239-4aa9-9105-48b19a9bb2cc")
	},
}
