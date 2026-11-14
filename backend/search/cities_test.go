package search

import (
	"slices"
	"testing"

	"conexiuni-cluj/router"
)

var testCities = []City{
	{Slug: "cluj", Name: "Cluj-Napoca", Operator: "CTP Cluj-Napoca"},
	{Slug: "iasi", Name: "Iași", Operator: "CTP Iași"},
	{Slug: "targumures", Name: "Târgu Mureș", Operator: "Transport Local"},
	{Slug: "galati", Name: "Galați", Operator: "Transurb Galați"},
	{Slug: "dej", Name: "Dej", Operator: "Transurb Dej"},
	{Slug: "sibiu", Name: "Sibiu", Operator: "Tursib"},
}

func TestNamedCity(t *testing.T) {
	cases := []struct {
		words []string
		slug  string
		rest  []string
	}{
		{router.Words("linia 26 tg mures"), "targumures", []string{"linia", "26"}},
		{router.Words("26 tirgu mures"), "targumures", []string{"26"}},
		{router.Words("traseu 105 galati"), "galati", []string{"traseu", "105"}},
		{router.Words("cluj"), "cluj", []string{}},
		{router.Words("targumures 3"), "targumures", []string{"3"}},
		{router.Words("piata unirii"), "", []string{"piata", "unirii"}},
	}
	for _, c := range cases {
		city, rest := namedCity(c.words, testCities)
		slug := ""
		if city != nil {
			slug = city.Slug
		}
		if slug != c.slug || !slices.Equal(rest, c.rest) && len(rest)+len(c.rest) > 0 {
			t.Errorf("namedCity(%v) = %q, %v", c.words, slug, rest)
		}
	}
}

func TestNamedOperator(t *testing.T) {
	found, rest := namedOperator([]string{"ctp", "29s"}, testCities, "iasi")
	if len(found) != 2 || found[0].Slug != "iasi" || !slices.Equal(rest, []string{"29s"}) {
		t.Errorf("ctp: %v %v", found, rest)
	}
	if found, _ := namedOperator([]string{"tursib", "5"}, testCities, "cluj"); len(found) != 1 || found[0].Slug != "sibiu" {
		t.Errorf("tursib: %v", found)
	}
	if found, _ := namedOperator([]string{"transport", "local"}, testCities, "cluj"); len(found) != 0 {
		t.Errorf("filler words named %v", found)
	}
}

func TestTownNamed(t *testing.T) {
	towns := []router.Town{{Name: "Piatra Neamț"}, {Name: "Turda"}}
	if town, ok := townNamed([]string{"piatra", "neamt"}, towns); !ok || town.Name != "Piatra Neamț" {
		t.Errorf("piatra neamt: %v %v", town, ok)
	}
	if _, ok := townNamed([]string{"piatra"}, towns); ok {
		t.Error("a part of a name is no town")
	}
}
