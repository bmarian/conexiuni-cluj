// Package arad is Arad: CTP Arad's network and buses from its Modeshift journey planner.
package arad

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "arad", Name: "Arad", Operator: "CTP Arad", OperatorURL: "https://ctparad.ro/", County: "arad",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Modeshift("https://www.telelink.city/api/v1/e39aa927-11c1-435f-92b9-57804f7bc1b4")
		f.News = env.RSS("https://ctparad.ro/category/comunicari/noutati-si-anunturi/feed/")
		return f, err
	},
}
