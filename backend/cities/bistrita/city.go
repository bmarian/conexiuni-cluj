// Package bistrita is Bistrița: Transmixt's network and buses, its county lines included, from its Modeshift journey planner.
package bistrita

import "conexiuni-cluj/cities"

var City = cities.Spec{
	Slug: "bistrita", Name: "Bistrița", Operator: "Transmixt", OperatorURL: "https://tmxbn.ro/", County: "bistrita-nasaud",
	Feeds: func(env *cities.Env) (cities.Feeds, error) {
		f, err := env.Modeshift("https://app.bistrita-transport.com/api/v1/0697fb4a-6530-4bf2-a143-98225b196a32")
		f.News = env.RSS("https://tmxbn.ro/feed/")
		return f, err
	},
}
