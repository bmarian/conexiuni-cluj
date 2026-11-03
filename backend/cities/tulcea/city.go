// Package tulcea is Tulcea: Transport Public Tulcea's network and buses from its Modeshift journey planner.
package tulcea

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "tulcea", Name: "Tulcea", Operator: "Transport Public Tulcea", OperatorURL: "https://stp-tulcea.ro/", County: "tulcea",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Modeshift("https://app.tulcea-transport.com/api/v1/774f6aa5-607b-4d46-8348-ab043cd194bd")
		f.News = env.RSS("https://stp-tulcea.ro/feed/")
		return f, err
	},
}
