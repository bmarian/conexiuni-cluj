// Package bocsa is Bocșa: the town's lines and buses from its Modeshift journey planner.
package bocsa

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "bocsa", Name: "Bocșa", Operator: "Bocșa Transport Public Local", OperatorURL: "https://bocsa-transport.telelink.city/", County: "caras-severin",
	// No news: the operator has no site of its own.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/3fb5935b-b8e1-4b32-ab1d-5253ec2e96f2")
	},
}
