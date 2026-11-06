package network

import (
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"math"
	"sort"
	"sync"
)

// A colour in OKLab, where distance follows how different two colours look.
type lab struct{ l, a, b float64 }

func (c lab) dist(o lab) float64 {
	return math.Sqrt((c.l-o.l)*(c.l-o.l) + (c.a-o.a)*(c.a-o.a) + (c.b-o.b)*(c.b-o.b))
}

func oklab(r, g, b float64) lab {
	r, g, b = models.Linear(r), models.Linear(g), models.Linear(b)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return lab{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// Channels fall outside 0..1 where sRGB cannot show the colour.
func oklchRGB(l, chroma, hue float64) (r, g, b float64) {
	a, bb := chroma*math.Cos(hue*math.Pi/180), chroma*math.Sin(hue*math.Pi/180)
	lc := math.Pow(l+0.3963377774*a+0.2158037573*bb, 3)
	mc := math.Pow(l-0.1055613458*a-0.0638541728*bb, 3)
	sc := math.Pow(l-0.0894841775*a-1.2914855480*bb, 3)
	gamma := func(v float64) float64 {
		if v <= 0.0031308 {
			return 12.92 * v
		}
		return 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return gamma(4.0767416621*lc - 3.3077115913*mc + 0.2309699292*sc),
		gamma(-1.2684380046*lc + 2.6097574011*mc - 0.3413193965*sc),
		gamma(-0.0041960863*lc - 0.7034186147*mc + 1.7076147010*sc)
}

// Dark shades must carry white numbers alone; light ones rely on the numbers' shadow.
var shades = []struct {
	l, maxChroma float64
	light        bool
}{{0.44, 0.19, false}, {0.52, 0.19, false}, {0.60, 0.19, false}, {0.78, 0.15, true}, {0.86, 0.12, true}}

const (
	hueStep   = 5
	minChroma = 0.06
)

type candidate struct {
	hex string
	lab lab
}

var palette = sync.OnceValue(func() []candidate {
	inGamut := func(v float64) bool { return v >= 0 && v <= 1 }
	var out []candidate
	for _, shade := range shades {
		for hue := 0; hue < 360; hue += hueStep {
			for chroma := shade.maxChroma; chroma >= minChroma; chroma -= 0.005 {
				r, g, b := oklchRGB(shade.l, chroma, float64(hue))
				if !inGamut(r) || !inGamut(g) || !inGamut(b) {
					continue
				}
				if hex := models.RGBHex(r, g, b); shade.light || models.WhiteReadable(hex) {
					out = append(out, candidate{hex, labOfHex(hex)})
				}
				break
			}
		}
	}
	return out
})

func labOfHex(hex string) lab {
	r, g, b := models.HexRGB(hex)
	return oklab(r, g, b)
}

type place struct{ lat, lon float64 }

// The stations each line stops at, platforms counted as their station.
func stationsOfLines(bundle *models.NetworkBundle) (map[string]map[int]bool, map[int]place) {
	stationOf := make(map[int]int, len(bundle.Stops))
	at := map[int]place{}
	for _, stop := range bundle.Stops {
		station := stop.Station()
		stationOf[stop.StopID] = station
		if _, ok := at[station]; !ok {
			at[station] = place{stop.StopLat, stop.StopLon}
		}
	}
	out := map[string]map[int]bool{}
	for _, route := range bundle.Routes {
		if out[route.RouteShortName] == nil {
			out[route.RouteShortName] = map[int]bool{}
		}
		for _, trip := range route.Trips {
			for _, stop := range trip.StopIDs {
				out[route.RouteShortName][stationOf[stop]] = true
			}
		}
	}
	return out, at
}

// Within the city's median gap to the nearest station, riders take two stations for one place.
func nearStations(at map[int]place) map[int][]int {
	ids := make([]int, 0, len(at))
	for id := range at {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if at[ids[i]].lat != at[ids[j]].lat {
			return at[ids[i]].lat < at[ids[j]].lat
		}
		return ids[i] < ids[j]
	})
	meters := func(i, j int) float64 {
		return geo.Haversine(at[ids[i]].lat, at[ids[i]].lon, at[ids[j]].lat, at[ids[j]].lon)
	}
	north := func(i, j int) float64 { return geo.Haversine(at[ids[i]].lat, 0, at[ids[j]].lat, 0) }
	// Sorted by latitude, so the north-south gap alone ends a scan.
	within := func(i int, limit float64, visit func(j int, d float64) float64) {
		for _, step := range [2]int{-1, 1} {
			for j := i + step; j >= 0 && j < len(ids) && north(i, j) <= limit; j += step {
				limit = visit(j, meters(i, j))
			}
		}
	}
	nearest := make([]float64, 0, len(ids))
	for i := range ids {
		best := math.Inf(1)
		within(i, best, func(_ int, d float64) float64 {
			best = min(best, d)
			return best
		})
		if !math.IsInf(best, 1) {
			nearest = append(nearest, best)
		}
	}
	out := map[int][]int{}
	if len(nearest) == 0 {
		return out
	}
	sort.Float64s(nearest)
	gap := nearest[len(nearest)/2]
	for i := range ids {
		within(i, gap, func(j int, d float64) float64 {
			if d <= gap {
				out[ids[i]] = append(out[ids[i]], ids[j])
			}
			return gap
		})
	}
	return out
}

// lineColors keeps lines that meet apart, each as near its ColorfulRouteColor as that allows.
func lineColors(stationsOf map[string]map[int]bool, near map[int][]int) map[string]string {
	linesAt := map[int][]string{}
	for name, stations := range stationsOf {
		for station := range stations {
			linesAt[station] = append(linesAt[station], name)
		}
	}
	meets := map[string]map[string]bool{}
	for name, stations := range stationsOf {
		meets[name] = map[string]bool{}
		for station := range stations {
			for _, there := range append([]int{station}, near[station]...) {
				for _, other := range linesAt[there] {
					if other != name {
						meets[name][other] = true
					}
				}
			}
		}
	}
	order := make([]string, 0, len(stationsOf))
	for name := range stationsOf {
		order = append(order, name)
	}
	sort.Slice(order, func(i, j int) bool {
		if len(meets[order[i]]) != len(meets[order[j]]) {
			return len(meets[order[i]]) > len(meets[order[j]])
		}
		return order[i] < order[j]
	})

	colors := palette()
	apart := make([][]float64, len(colors))
	for i := range colors {
		apart[i] = make([]float64, len(colors))
		for j := range colors {
			apart[i][j] = colors[i].lab.dist(colors[j].lab)
		}
	}
	// Whole thousandths, so near-equal choices settle the same way every build.
	thousandths := func(v float64) int { return int(math.Round(v * 1000)) }
	picked := map[string]int{}
	choose := func(name string) int {
		own := labOfHex(models.ColorfulRouteColor(name))
		best, bestRoom, bestNear := 0, -1, math.MaxInt
		for i, c := range colors {
			room := math.MaxInt
			for other := range meets[name] {
				if j, ok := picked[other]; ok {
					room = min(room, thousandths(apart[i][j]))
				}
			}
			nearOwn := thousandths(c.lab.dist(own))
			if room > bestRoom || room == bestRoom && nearOwn < bestNear {
				best, bestRoom, bestNear = i, room, nearOwn
			}
		}
		return best
	}
	for _, name := range order {
		picked[name] = choose(name)
	}
	// Later lines may have taken room an earlier one needed: settle until no line moves.
	for range len(order) {
		moved := false
		for _, name := range order {
			if c := choose(name); c != picked[name] {
				picked[name], moved = c, true
			}
		}
		if !moved {
			break
		}
	}
	out := make(map[string]string, len(order))
	for _, name := range order {
		out[name] = colors[picked[name]].hex
	}
	return out
}
