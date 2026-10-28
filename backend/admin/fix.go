package admin

import (
	"cmp"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/database"
	"conexiuni-cluj/live"
	"conexiuni-cluj/planner"
	"slices"
	"strconv"
	"strings"
)

const adminFixDays = 7

type fixSearch struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

type fixPlan struct {
	Reason string `json:"reason"`
	From   string `json:"from"`
	To     string `json:"to"`
	Count  int    `json:"count"`
}

type fixRoute struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type fixLine struct {
	fixRoute
	N      int `json:"n"`
	Off    int `json:"off"`
	AbsSec int `json:"avg_abs_sec"`
}

type fixBus struct {
	fixRoute
	Reason string   `json:"reason"`
	Buses  []string `json:"buses"`
	Count  int      `json:"count"`
	Share  float64  `json:"share"`
}

type adminFix struct {
	Searches []fixSearch `json:"searches"`
	Plans    []fixPlan   `json:"plans"`
	Lines    []fixLine   `json:"lines"`
	Buses    []fixBus    `json:"buses"`
}

func fixSection(city *cities.City) (adminFix, error) {
	since := database.StatsDaysAgo(adminFixDays - 1)
	f := adminFix{}
	var err error
	if f.Searches, err = fixSearches(city, since); err != nil {
		return f, err
	}
	if f.Plans, err = fixPlans(city, since); err != nil {
		return f, err
	}
	if f.Lines, err = fixLines(city, since); err != nil {
		return f, err
	}
	f.Buses, err = fixBuses(city, since)
	return f, err
}

func metricTotals(city *cities.City, metric, since string) (map[string]int, error) {
	rows, err := city.MetricRows(metric, since)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, r := range rows {
		out[r.Key] += r.Count
	}
	return out, nil
}

func fixSearches(city *cities.City, since string) ([]fixSearch, error) {
	counts, err := metricTotals(city, planner.PlaceMissMetric, since)
	if err != nil {
		return nil, err
	}
	out := []fixSearch{}
	for q, n := range counts {
		typing := false
		for other := range counts {
			if len(other) > len(q) && strings.HasPrefix(other, q) {
				typing = true
				break
			}
		}
		if !typing {
			out = append(out, fixSearch{Query: q, Count: n})
		}
	}
	return top(out, func(a, b fixSearch) int { return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Query, b.Query)) }), nil
}

func fixPlans(city *cities.City, since string) ([]fixPlan, error) {
	counts, err := metricTotals(city, planner.PlanMissMetric, since)
	if err != nil {
		return nil, err
	}
	out := []fixPlan{}
	for key, n := range counts {
		parts := strings.SplitN(key, "|", 3)
		if len(parts) != 3 {
			continue
		}
		out = append(out, fixPlan{Reason: parts[0], From: parts[1], To: parts[2], Count: n})
	}
	return top(out, func(a, b fixPlan) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.From+a.To, b.From+b.To))
	}), nil
}

func fixLines(city *cities.City, since string) ([]fixLine, error) {
	counts, err := metricTotals(city, live.LineAccuracyMetric, since)
	if err != nil {
		return nil, err
	}
	byRoute := map[int]*fixLine{}
	abs := map[int]int{}
	for key, n := range counts {
		raw, field, ok := strings.Cut(key, "|")
		id, err := strconv.Atoi(raw)
		if !ok || err != nil {
			continue
		}
		l := byRoute[id]
		if l == nil {
			l = &fixLine{fixRoute: fixRoute{ID: id}}
			byRoute[id] = l
		}
		switch field {
		case "n":
			l.N += n
		case "off":
			l.Off += n
		case "abs":
			abs[id] += n
		}
	}
	lines := make([]fixLine, 0, len(byRoute))
	for id, l := range byRoute {
		if l.Off == 0 || l.N == 0 {
			continue
		}
		l.AbsSec = abs[id] / l.N
		lines = append(lines, *l)
	}
	lines = top(lines, func(a, b fixLine) int { return cmp.Or(cmp.Compare(b.Off, a.Off), cmp.Compare(a.ID, b.ID)) })
	routes := make([]*fixRoute, len(lines))
	for i := range lines {
		routes[i] = &lines[i].fixRoute
	}
	return lines, nameRoutes(city, routes)
}

func fixBuses(city *cities.City, since string) ([]fixBus, error) {
	counts, err := metricTotals(city, live.UnplacedMetric, since)
	if err != nil {
		return nil, err
	}
	seen, err := metricTotals(city, live.SeenMetric, since)
	if err != nil {
		return nil, err
	}
	type group struct{ reason, route string }
	groups := map[group]*fixBus{}
	for key, n := range counts {
		parts := strings.SplitN(key, "|", 3)
		if len(parts) != 3 {
			continue
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		g := group{parts[0], parts[1]}
		b := groups[g]
		if b == nil {
			b = &fixBus{fixRoute: fixRoute{ID: id}, Reason: parts[0]}
			groups[g] = b
		}
		b.Buses = append(b.Buses, parts[2])
		b.Count += n
	}
	out := make([]fixBus, 0, len(groups))
	for g, b := range groups {
		slices.SortFunc(b.Buses, func(x, y string) int { return cmp.Or(compareVersions(x, y), cmp.Compare(x, y)) })
		if n := seen[g.route]; n > 0 {
			b.Share = min(1, float64(b.Count)/float64(n))
		}
		out = append(out, *b)
	}
	out = top(out, func(a, b fixBus) int {
		return cmp.Or(cmp.Compare(b.Share, a.Share), cmp.Compare(len(b.Buses), len(a.Buses)), cmp.Compare(b.Count, a.Count), cmp.Compare(a.ID, b.ID))
	})
	routes := make([]*fixRoute, len(out))
	for i := range out {
		routes[i] = &out[i].fixRoute
	}
	return out, nameRoutes(city, routes)
}

func top[T any](rows []T, compare func(a, b T) int) []T {
	slices.SortFunc(rows, compare)
	return rows[:min(len(rows), adminTopLimit)]
}

func nameRoutes(city *cities.City, routes []*fixRoute) error {
	ids := make([]int, len(routes))
	for i, r := range routes {
		ids[i] = r.ID
	}
	names, err := routeNames(city, ids)
	for _, r := range routes {
		r.Name, r.Color = names[r.ID].Name, names[r.ID].Color
		if r.Name == "" {
			r.Name = strconv.Itoa(r.ID)
		}
	}
	return err
}
