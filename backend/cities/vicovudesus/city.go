// Package vicovudesus is Vicovu de Sus: the town's lines and buses from its Modeshift journey planner.
package vicovudesus

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "vicovudesus", Name: "Vicovu de Sus", Operator: "Orașul Vicovu de Sus", OperatorURL: "https://primaria-vicovudesus.ro/servicii-publice/transport-public/", County: "suceava",
	// No news: the town hall posts no transport news.
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.modeshift.app/api/v1/cd769aa9-9b77-48e6-9f52-acb323f351d5")
	},
}
