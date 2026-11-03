// Package drobeta is Drobeta-Turnu Severin: TPU Drobeta's network and buses from its Modeshift journey planner.
package drobeta

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "drobeta", Name: "Drobeta-Turnu Severin", Operator: "TPU Drobeta", OperatorURL: "https://drobeta-transport.telelink.city/", County: "mehedinti",
	// No news: TPU Drobeta has no site of its own.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/e2c003d7-ca61-4c59-938a-048fdbf56ed1")
	},
}
