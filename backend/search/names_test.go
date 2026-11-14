package search

import (
	"testing"

	"conexiuni-cluj/router"
)

// Same cases as frontend/src/utils/__tests__/nameMatching.spec.ts.
func TestNameMatches(t *testing.T) {
	cases := []struct {
		name, query string
		typos, want bool
	}{
		{"Bulevardul 21 Decembrie 1989", "decembrie 21", false, true},
		{"P-ta M. Viteazu", "piata mihai viteazu", false, true},
		{"P-ța Unirii", "piata unirii", false, true},
		{"C-tin Brâncoveanu Est", "constantin brancoveanu", false, true},
		{"B-dul Muncii", "bulevardul muncii", false, true},
		{"M.Viteazu", "mihai viteazu", false, true},
		{"Str. Unirii", "strada unirii", false, true},
		{"M-Școala Gimnazială", "scoala", false, true},
		{"Horea", "strada horea", false, true},
		{"Horea", "bd horea", false, true},
		{"P-ta M. Viteazu", "marasti", false, false},
		{"Str. Unirii", "strazi", false, false},
		{"Iulius Mall Est", "iulius vest", false, false},
		{"Iulius Mall Est", "iulus", false, false},
		{"Iulius Mall Est", "iulus", true, true},
		{"Memorandumului Sud", "memorandomulu", true, true},
		{"Gara Mare", "gar", true, true},
		{"Mare", "mar", true, true},
		{"Bar", "car", true, false},
		{"Linia 25", "24", true, false},
		{"Scoala 2019", "2018", true, false},
	}
	for _, c := range cases {
		if got := nameMatches(router.Fold(c.name), router.Fold(c.query), c.typos); got != c.want {
			t.Errorf("nameMatches(%q, %q, %v) = %v", c.name, c.query, c.typos, got)
		}
	}
}

func TestMatchRank(t *testing.T) {
	if r := matchRank("1 decembrie (tunari)", "1 decembrie"); r != 0 {
		t.Errorf("start: %d", r)
	}
	if r := matchRank("bd. 1 decembrie 1918 (strada liviu rebreanu)", "1 decembrie"); r != 1 {
		t.Errorf("own name: %d", r)
	}
	if r := matchRank("liviu rebreanu (bulevardul 1 decembrie 1918, bucuresti)", "1 decembrie"); r != 2 {
		t.Errorf("town part: %d", r)
	}
}

func TestMatchingTriesTyposLast(t *testing.T) {
	name := func(s string) string { return router.Fold(s) }
	got := matching([]string{"Iulius Mall", "Iulis"}, name, "iulis")
	if len(got) != 1 || got[0].item != "Iulis" {
		t.Errorf("exact first: %v", got)
	}
	got = matching([]string{"Iulius Mall"}, name, "iulis")
	if len(got) != 1 || got[0].rank != wordsRank {
		t.Errorf("typo: %v", got)
	}
}
