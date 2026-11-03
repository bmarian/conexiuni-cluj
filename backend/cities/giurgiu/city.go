// Package giurgiu is Giurgiu: Tracum's network and buses from its Modeshift journey planner.
package giurgiu

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "giurgiu", Name: "Giurgiu", Operator: "Tracum Giurgiu", OperatorURL: "https://giurgiu-transport.telelink.city/", County: "giurgiu",
	// No news: Tracum has no site of its own.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/930ce067-444d-4fa4-996d-d754daca6f01")
	},
}
