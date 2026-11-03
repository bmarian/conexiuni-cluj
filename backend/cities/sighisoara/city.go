// Package sighisoara is Sighișoara: Sighișoara Transport Local's network and buses from its Modeshift journey planner.
package sighisoara

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "sighisoara", Name: "Sighișoara", Operator: "Sighișoara Transport Local", OperatorURL: "https://sighisoara-transport.modeshift.com/", County: "mures",
	// No news: the operator has no site of its own.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.modeshift.app/api/v1/699bbdf8-c665-4bb3-ae02-59cebd8c973e")
	},
}
