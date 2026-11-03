// Package dej is Dej: Transurb Dej's network and buses from its Modeshift journey planner.
package dej

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "dej", Name: "Dej", Operator: "Transurb Dej", OperatorURL: "https://transurbdej.ro/", County: "cluj",
	// No news: transurbdej.ro's feed is empty.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://app.dej-transport.com/api/v1/a98ca685-ae59-4d03-82fd-adaba14f6ab5")
	},
}
