// Package braila is Brăila: Braicar's network and buses from its Modeshift journey planner.
package braila

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "braila", Name: "Brăila", Operator: "Braicar", OperatorURL: "https://www.braicar.ro/", County: "braila",
	// No news: braicar.ro has no feed.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/74c48b41-6865-4be2-a305-639baa7ae148")
	},
}
