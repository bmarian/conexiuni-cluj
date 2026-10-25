package network

import (
	"conexiuni-cluj/models"
	"math"
	"sort"
)

// A colour in OKLab, where distance follows how different two colours look.
type lab struct{ l, a, b float64 }

func (c lab) dist(o lab) float64 {
	return math.Sqrt((c.l-o.l)*(c.l-o.l) + (c.a-o.a)*(c.a-o.a) + (c.b-o.b)*(c.b-o.b))
}

func oklab(r, g, b float64) lab {
	linear := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	r, g, b = linear(r), linear(g), linear(b)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return lab{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// By whole degree of hue.
type hueCircle [360]lab

func circleOf(s, l float64) *hueCircle {
	var c hueCircle
	for h := range c {
		c[h] = oklab(models.HSLRGB(float64(h), s, models.ReadableLightness(float64(h), s, l)))
	}
	return &c
}

// How far apart n lines at one station can be at best: n colours spread evenly around the circle.
func (c *hueCircle) spread(n int) float64 {
	length := 0.0
	for h := range c {
		length += c[h].dist(c[(h+1)%360])
	}
	return length / math.Pi * math.Sin(math.Pi/float64(n))
}

// The stations each line stops at, platforms counted as their station.
func stationsOfLines(bundle *models.NetworkBundle) map[string]map[int]bool {
	stationOf := make(map[int]int, len(bundle.Stops))
	for _, stop := range bundle.Stops {
		stationOf[stop.StopID] = stop.Station()
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
	return out
}

// lineColors keeps lines that share a station apart, each as near its ColorfulRouteColor as that allows.
func lineColors(stationsOf map[string]map[int]bool) map[string]string {
	linesAt := map[int][]string{}
	for name, stations := range stationsOf {
		for station := range stations {
			linesAt[station] = append(linesAt[station], name)
		}
	}
	// For each pair of lines, the fewest lines any station they share has.
	together := map[string]map[string]int{}
	for _, names := range linesAt {
		for _, a := range names {
			for _, b := range names {
				if a == b {
					continue
				}
				if together[a] == nil {
					together[a] = map[string]int{}
				}
				if together[a][b] == 0 || len(names) < together[a][b] {
					together[a][b] = len(names)
				}
			}
		}
	}
	// A pair that also meets at a quiet stop needs no more room than a typical station's lines get.
	sizes := []int{}
	for _, names := range linesAt {
		if len(names) > 1 {
			sizes = append(sizes, len(names))
		}
	}
	sort.Ints(sizes)
	typical := 2
	if len(sizes) > 0 {
		typical = sizes[len(sizes)/2]
	}
	for a := range together {
		for b, count := range together[a] {
			together[a][b] = max(count, typical)
		}
	}
	order := make([]string, 0, len(stationsOf))
	for name := range stationsOf {
		order = append(order, name)
	}
	sort.Slice(order, func(i, j int) bool {
		if len(together[order[i]]) != len(together[order[j]]) {
			return len(together[order[i]]) > len(together[order[j]])
		}
		return order[i] < order[j]
	})

	lo, hi := models.ColorfulLightness()
	type line struct {
		own, hue, shade int
		s               float64
		shades          [4]float64
		circles         [4]*hueCircle
	}
	lines := map[string]*line{}
	for _, name := range order {
		if h, s, l, ok := models.ColorfulHSL(name); ok {
			// One step darker too: made readable, light colours crowd together and need the room.
			ln := &line{own: int(h), hue: -1, s: s, shades: [4]float64{l, lo, hi, lo - (hi - lo)}}
			for i, shade := range ln.shades {
				ln.circles[i] = circleOf(s, shade)
			}
			lines[name] = ln
		}
	}
	place := func(name string) bool {
		ln := lines[name]
		var near []apart
		for other, count := range together[name] {
			if o := lines[other]; o != nil && o.hue >= 0 {
				near = append(near, apart{o.circles[o.shade][o.hue], ln.circles[0].spread(count)})
			}
		}
		hue, shade := pickHue(ln.own, ln.circles, near)
		moved := hue != ln.hue || shade != ln.shade
		ln.hue, ln.shade = hue, shade
		return moved
	}
	for _, name := range order {
		if lines[name] != nil {
			place(name)
		}
	}
	// Later lines may have taken room an earlier one needed: settle until no line moves.
	for range len(order) {
		moved := false
		for _, name := range order {
			if lines[name] != nil && place(name) {
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	out := make(map[string]string, len(order))
	for _, name := range order {
		if ln := lines[name]; ln != nil {
			hue := float64(ln.hue)
			out[name] = models.HSLHex(hue, ln.s, models.ReadableLightness(hue, ln.s, ln.shades[ln.shade]))
		} else {
			out[name] = models.FallbackRouteColor
		}
	}
	return out
}

type apart struct {
	color lab
	need  float64
}

// The hue and shade nearest own with room from every neighbour, else the roomiest.
func pickHue(own int, circles [4]*hueCircle, near []apart) (hue, shade int) {
	bestHue, bestShade, room := own, 0, math.Inf(-1)
	for step := 0; step <= 180; step++ {
		for _, h := range [2]int{(own + step) % 360, (own - step + 360) % 360} {
			for sh, circle := range circles {
				least := math.Inf(1)
				for _, n := range near {
					least = min(least, circle[h].dist(n.color)/n.need)
				}
				if least >= 1 {
					return h, sh
				}
				if least > room {
					bestHue, bestShade, room = h, sh, least
				}
			}
		}
	}
	return bestHue, bestShade
}
