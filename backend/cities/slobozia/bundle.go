package slobozia

import (
	"cmp"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/live"
	"conexiuni-cluj/router"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// One of SPTL's routes as sptl.ro's bundle holds it, its stops SPTL's keys.
type route struct {
	id, line string
	stops    []string
	// The stop a loop waits at before heading back; 0 on a route with an end of its own.
	dwell int
	// From the first stop to each; nil when the bundle has none for the route.
	minutes []int
	// By SPTL's kind of day: when runs leave the first stop and, on a loop, the stop it waits at.
	leaves, turns map[string][]int
	// The median minutes a loop's runs take from that stop back to the first.
	rideBack int
}

// A row of a timetable: one round from the start to the far end and back.
type row struct {
	day               string
	leave, turn, ride int
	detour            bool
}

// SPTL's kinds of day; a table picked without one is read for weekdays.
const (
	weekdays = "wd"
	weekends = "we"
)

// SPTL names a way back from the far end "…_RETUR"; it leaves when its rows leave the far end.
const backSuffix = "_RETUR"

const minutesPerDay = 24 * 60

var (
	stopEntry  = regexp.MustCompile(`(\w+):\{name:("(?:[^"\\]|\\.)*"),lat:(-?[\d.]+),lng:(-?[\d.]+)\}`)
	routeStart = regexp.MustCompile(`\{id:"`)
	routeField = regexp.MustCompile(`[{,](id|line):"([^"]*)"`)
	dwellField = regexp.MustCompile(`[{,]dwellStopIdx:(\d+)`)
	stopsField = regexp.MustCompile(`[{,]stops:\[([^\]]*)\]`)
	// The times written into a route, as wd:[e(4,30),e(18,7,25),…]: hour, minute and the ride.
	dayField  = regexp.MustCompile(`[{,](\w+):\[((?:[\w$]+\([\d,]*\),?)*)\]`)
	departure = regexp.MustCompile(`[\w$]+\((\d+),(\d+)(?:,\d+)?\)`)
	// The timetables, as template literals headed day_type,pg_dep,out_dur,far_arr,dwell,far_dep,ret_dur,pg_arr,n[,ocol].
	table = regexp.MustCompile("([\\w$]+)=`(?:\\\\uFEFF)?(day_type,[^`]*)`")
	// Each route's rows, as ROUTE:pick(TABLE[,"we"][,!0|!1]): !0 only the runs with the detour, !1 only those without.
	pick         = regexp.MustCompile(`[{,](\w+):[\w$]+\(([\w$]+)(?:,"(\w+)")?(?:,(!0|!1))?\)`)
	minutesList  = regexp.MustCompile(`=\{((?:\w+:\[[\d.,]*\],?)+)\}`)
	minutesEntry = regexp.MustCompile(`(\w+):\[([\d.,]*)\]`)
)

// parseBundle reads the stops and routes; a route's runs come from its timetable rows, else from the times written into it, as on the site.
func parseBundle(data []byte) (map[string]sitegtfs.Stop, []route, error) {
	src := string(data)
	stops := map[string]sitegtfs.Stop{}
	for _, m := range stopEntry.FindAllStringSubmatch(src, -1) {
		name, errName := strconv.Unquote(m[2])
		lat, errLat := strconv.ParseFloat(m[3], 64)
		lon, errLon := strconv.ParseFloat(m[4], 64)
		if errName == nil && errLat == nil && errLon == nil {
			stops[m[1]] = sitegtfs.Stop{ID: m[1], Name: name, At: router.Point{Lat: lat, Lon: lon}, Placed: true}
		}
	}
	var routes []route
	for _, at := range routeStart.FindAllStringIndex(src, -1) {
		if r, ok := parseRoute(object(src, at[0])); ok {
			routes = append(routes, r)
		}
	}
	if len(stops) == 0 || len(routes) == 0 {
		return nil, nil, fmt.Errorf("%d stops and %d routes in the bundle", len(stops), len(routes))
	}
	byID := map[string]*route{}
	minutes := minutesOf(src)
	for i := range routes {
		r := &routes[i]
		byID[r.id] = r
		if m := minutes[r.id]; len(m) == len(r.stops) {
			r.minutes = m
		}
	}
	tables := tablesOf(src)
	rides := map[string][]float64{}
	for _, m := range pick.FindAllStringSubmatch(src, -1) {
		r, rows := byID[m[1]], tables[m[2]]
		if r == nil || rows == nil {
			continue
		}
		day := cmp.Or(m[3], weekdays)
		var leaves, turns []int
		for _, row := range rows {
			if row.day != day || (m[4] != "" && row.detour != (m[4] == "!0")) {
				continue
			}
			leave := row.leave
			if strings.HasSuffix(r.id, backSuffix) {
				leave = row.turn
			}
			if leave >= 0 {
				leaves = append(leaves, leave)
			}
			if row.turn >= 0 {
				turns = append(turns, row.turn)
			}
			if row.ride >= 0 {
				rides[r.id] = append(rides[r.id], float64(row.ride))
			}
		}
		if len(leaves) > 0 {
			r.leaves[day], r.turns[day] = leaves, turns
		}
	}
	for id, ride := range rides {
		byID[id].rideBack = int(math.Round(live.Median(ride)))
	}
	return stops, routes, nil
}

func parseRoute(obj string) (route, bool) {
	r := route{leaves: map[string][]int{}, turns: map[string][]int{}}
	for _, m := range routeField.FindAllStringSubmatch(obj, -1) {
		if m[1] == "id" {
			r.id = m[2]
		} else {
			r.line = m[2]
		}
	}
	if m := stopsField.FindStringSubmatch(obj); m != nil {
		for _, s := range strings.Split(m[1], ",") {
			if key, err := strconv.Unquote(s); err == nil {
				r.stops = append(r.stops, key)
			}
		}
	}
	if m := dwellField.FindStringSubmatch(obj); m != nil {
		if at, _ := strconv.Atoi(m[1]); at < len(r.stops)-1 {
			r.dwell = at
		}
	}
	for _, m := range dayField.FindAllStringSubmatch(obj, -1) {
		if _, ok := kinds[m[1]]; !ok {
			continue
		}
		for _, d := range departure.FindAllStringSubmatch(m[2], -1) {
			h, _ := strconv.Atoi(d[1])
			minute, _ := strconv.Atoi(d[2])
			r.leaves[m[1]] = append(r.leaves[m[1]], h*60+minute)
		}
	}
	return r, r.id != "" && r.line != "" && len(r.stops) >= 2
}

// The object literal opening at start; braces in strings do not count.
func object(src string, start int) string {
	depth := 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '"':
			for i++; i < len(src) && src[i] != '"'; i++ {
				if src[i] == '\\' {
					i++
				}
			}
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return src[start : i+1]
			}
		}
	}
	return ""
}

// The rows of each table by its name in the bundle; a start's columns are named after it (pg_ Piața Gării, kf_ Kaufland).
func tablesOf(src string) map[string][]row {
	out := map[string][]row{}
	for _, m := range table.FindAllStringSubmatch(src, -1) {
		lines := strings.Split(strings.ReplaceAll(m[2], `\r`, ""), "\n")
		col := map[string]int{}
		for i, name := range strings.Split(strings.TrimSpace(lines[0]), ",") {
			if strings.HasSuffix(name, "_dep") && name != "far_dep" {
				name = "start_dep"
			}
			col[name] = i
		}
		var rows []row
		for _, line := range lines[1:] {
			fields := strings.Split(strings.TrimSpace(line), ",")
			get := func(name string) string {
				if i, ok := col[name]; ok && i < len(fields) {
					return strings.TrimSpace(fields[i])
				}
				return ""
			}
			r := row{day: get("day_type"), leave: clock(get("start_dep")), turn: clock(get("far_dep")), ride: count(get("ret_dur")), detour: get("ocol") == "1"}
			if r.day == "" {
				continue
			}
			// Leaving the far end after midnight.
			if r.turn >= 0 && r.turn < r.leave {
				r.turn += minutesPerDay
			}
			rows = append(rows, r)
		}
		out[m[1]] = rows
	}
	return out
}

// The routes' minutes from their first stop: the list naming the most routes (another names a few loops' longer weekend waits, which splitting a loop where it waits makes needless).
func minutesOf(src string) map[string][]int {
	var best map[string][]int
	for _, list := range minutesList.FindAllStringSubmatch(src, -1) {
		got := map[string][]int{}
		for _, e := range minutesEntry.FindAllStringSubmatch(list[1], -1) {
			var minutes []int
			for _, v := range strings.Split(e[2], ",") {
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					minutes = nil
					break
				}
				minutes = append(minutes, int(math.Round(f)))
			}
			got[e[1]] = minutes
		}
		if len(got) > len(best) {
			best = got
		}
	}
	return best
}

// Minutes since midnight of "HH:MM"; -1 when blank or unreadable.
func clock(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

func count(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
