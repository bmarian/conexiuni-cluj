// Package vaslui is Vaslui: Transurb Vaslui's network and buses from its Modeshift journey planner.
package vaslui

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "vaslui", Name: "Vaslui", Operator: "Transurb Vaslui", OperatorURL: "https://transurb.ro/", County: "vaslui",
	// No news: transurb.ro's feed holds only WordPress's sample post.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/c74520e9-1016-4ec4-9302-3e2a525559b7")
	},
}
