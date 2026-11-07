package albaiulia

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/sitegtfs"
	"regexp"
	"sort"
	"strings"
)

var (
	// STP tells a road's two sides apart by a closing T or R, as in "Seliste T" and "Benic 1R".
	sideMark = regexp.MustCompile(`^(.*[\s\d.])\s*[RT]$`)
	// And the stands of lines 103 and 104 by the line, as in "Unirea(L103)".
	lineMark = regexp.MustCompile(`\s*\([Ll]?\d+\)$`)
	number   = regexp.MustCompile(`^\d+`)
)

// The name the app shows, so both sides of a road make one station.
func stationName(name string) string {
	name = sideMark.ReplaceAllString(strings.TrimSpace(name), "$1")
	return strings.TrimSpace(lineMark.ReplaceAllString(strings.TrimSpace(name), ""))
}

func stationKey(name string) string { return feeds.NormalizeStopLabel(stationName(name)) }

// A return (STP's label ends in R, or repeats the outward one) joins the outward route of its number whose ends it swaps and with which it shares the most stations.
func lines(pages []page) []sitegtfs.Line {
	type pair struct {
		out, back int
		shared    float64
	}
	var pairs []pair
	for b := range pages {
		for o := range pages {
			if o != b && returns(pages[o], pages[b], o, b) && mirrored(pages[o], pages[b]) {
				pairs = append(pairs, pair{o, b, shared(pages[o], pages[b])})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].shared > pairs[j].shared })
	used := make([]bool, len(pages))
	backOf := map[int]int{}
	for _, p := range pairs {
		if !used[p.out] && !used[p.back] {
			used[p.out], used[p.back] = true, true
			backOf[p.out] = p.back
		}
	}
	var out []sitegtfs.Line
	for i := range pages {
		switch b, ok := backOf[i]; {
		case ok:
			out = append(out, sitegtfs.Line{Name: pairName(pages[i].label, pages[b].label), Ways: [2]*sitegtfs.Route{&pages[i].Route, &pages[b].Route}})
		case !used[i]:
			out = append(out, sitegtfs.Line{Name: pages[i].label, Ways: [2]*sitegtfs.Route{&pages[i].Route, nil}})
		}
	}
	return out
}

func returns(out, back page, o, b int) bool {
	if number.FindString(out.label) != number.FindString(back.label) {
		return false
	}
	if out.label == back.label {
		return o < b
	}
	return strings.HasSuffix(back.label, "R") && !strings.HasSuffix(out.label, "R")
}

func mirrored(out, back page) bool {
	first, last := func(p page) string { return stationKey(p.Stops[0].Name) }, func(p page) string { return stationKey(p.Stops[len(p.Stops)-1].Name) }
	return first(back) == last(out) || last(back) == first(out)
}

// The share of the two routes' stations that both serve.
func shared(a, b page) float64 {
	both, all := 0, map[string]int{}
	for _, p := range []page{a, b} {
		seen := map[string]bool{}
		for _, s := range p.Stops {
			if key := stationKey(s.Name); !seen[key] {
				seen[key] = true
				all[key]++
			}
		}
	}
	for _, n := range all {
		if n == 2 {
			both++
		}
	}
	return float64(both) / float64(len(all))
}

// "101T" and "101R" are line 101 out and back.
func pairName(out, back string) string {
	if stem, ok := strings.CutSuffix(out, "T"); ok && back == stem+"R" {
		return stem
	}
	return out
}
