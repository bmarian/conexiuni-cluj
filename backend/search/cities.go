package search

import (
	"slices"
	"strings"

	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
)

type City struct {
	Slug, Name, Operator, OperatorURL string
	Network                           func() (*models.NetworkBundle, string, error)
}

// Spellings people use for the same word of a city's name.
var cityWordForms = map[string]string{"tg": "targu", "tirgu": "targu", "tirgoviste": "targoviste", "rm": "ramnicu", "rimnicu": "ramnicu"}

// Words of operator names that name no one.
var operatorFiller = wordSet("transport", "public", "publice", "local", "urban", "servicii", "municipale", "orasul", "sa", "srl")

func cityForm(w string) string {
	if form, ok := cityWordForms[w]; ok {
		return form
	}
	return w
}

func cityWords(name string) []string {
	words := router.Words(name)
	for i, w := range words {
		words[i] = cityForm(w)
	}
	return words
}

func wordsAt(words, name []string) int {
	for i := 0; i+len(name) <= len(words); i++ {
		if slices.Equal(words[i:i+len(name)], name) {
			return i
		}
	}
	return -1
}

func without(words []string, at, n int) []string {
	return slices.Concat(words[:at], words[at+n:])
}

// By all of its name, its slug, or a first word no other city starts with.
func namedCity(words []string, cities []City) (*City, []string) {
	forms := make([]string, len(words))
	for i, w := range words {
		forms[i] = cityForm(w)
	}
	firsts := map[string]int{}
	for _, c := range cities {
		firsts[cityWords(c.Name)[0]]++
	}
	for i := range cities {
		c := &cities[i]
		name := cityWords(c.Name)
		if at := wordsAt(forms, name); at >= 0 {
			return c, without(words, at, len(name))
		}
		if at := slices.Index(forms, c.Slug); at >= 0 {
			return c, without(words, at, 1)
		}
		if len(name) > 1 && firsts[name[0]] == 1 {
			if at := slices.Index(forms, name[0]); at >= 0 {
				return c, without(words, at, 1)
			}
		}
	}
	return nil, words
}

func operatorWords(c City) []string {
	own := cityWords(c.Name)
	var out []string
	for _, w := range router.Words(c.Operator) {
		if !operatorFiller[w] && !slices.Contains(own, cityForm(w)) {
			out = append(out, w)
		}
	}
	return out
}

// Here first.
func namedOperator(words []string, cities []City, here string) ([]*City, []string) {
	for _, w := range words {
		var found []*City
		for i := range cities {
			if slices.Contains(operatorWords(cities[i]), w) {
				found = append(found, &cities[i])
			}
		}
		if len(found) > 0 {
			slices.SortStableFunc(found, func(a, b *City) int {
				switch {
				case a.Slug == here:
					return -1
				case b.Slug == here:
					return 1
				}
				return 0
			})
			return found, slices.DeleteFunc(slices.Clone(words), func(x string) bool { return x == w })
		}
	}
	return nil, words
}

func lineNamed(bundle *models.NetworkBundle, words []string) (*models.NetworkRoute, []string) {
	for i, w := range words {
		for j := range bundle.Routes {
			if strings.EqualFold(bundle.Routes[j].RouteShortName, w) {
				return &bundle.Routes[j], without(words, i, 1)
			}
		}
	}
	return nil, words
}

func townNamed(words []string, towns []router.Town) (router.Town, bool) {
	forms := make([]string, len(words))
	for i, w := range words {
		forms[i] = cityForm(w)
	}
	for _, t := range towns {
		if slices.Equal(cityWords(t.Name), forms) {
			return t, true
		}
	}
	return router.Town{}, false
}
