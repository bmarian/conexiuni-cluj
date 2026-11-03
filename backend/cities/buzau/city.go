// Package buzau is Buzău: Trans Bus's network and buses from its Modeshift journey planner.
package buzau

import "conexiuni-cluj/cities"

var City = cities.Spec{
	// transbusbuzau.ro carries injected spam, so the operator links its journey planner; no news for the same reason.
	Slug: "buzau", Name: "Buzău", Operator: "Trans Bus Buzău", OperatorURL: "https://buzau-transport.com/", County: "buzau",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		return env.Modeshift("https://www.telelink.city/api/v1/556cb222-5456-403c-904f-cff58fbfd622")
	},
}
