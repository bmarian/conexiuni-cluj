package slobozia

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/follow"
	"conexiuni-cluj/models"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type realtime struct {
	url      string
	trace    feeds.Tracing
	follower *follow.Follower
}

func newRealtime(url string, ids feeds.IDMap, trace feeds.Tracing) *realtime {
	return &realtime{url: url, trace: trace, follower: follow.New(ids, trace)}
}

func (s *realtime) Budget() feeds.Budget { return feeds.Budget{} }

func (s *realtime) Vehicles() ([]models.Vehicle, error) {
	data, err := feeds.Download(s.url)
	if err != nil {
		return nil, fmt.Errorf("sptl.ro: %w", err)
	}
	now := time.Now()
	buses, err := parseBuses(data, s.trace.RouteIDs(), now)
	if err != nil {
		return nil, fmt.Errorf("sptl.ro: %w", err)
	}
	return s.follower.Vehicles(buses, now)
}

var errStale = errors.New("the live map's positions are stale")

// The map's buses by plate: "1" latitude, "2" longitude, "3" on or off, "11" the line on the display; age_s how old the positions are.
func parseBuses(data []byte, routes map[string]int, now time.Time) ([]follow.Bus, error) {
	var feed struct {
		Age     float64 `json:"age_s"`
		Stale   bool    `json:"stale"`
		Current map[string]struct {
			Lat     string `json:"1"`
			Lon     string `json:"2"`
			State   string `json:"3"`
			Display string `json:"11"`
		} `json:"current"`
	}
	if err := json.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	if feed.Stale {
		return nil, errStale
	}
	at := now.Add(-time.Duration(feed.Age * float64(time.Second)))
	var out []follow.Bus
	for plate, b := range feed.Current {
		routeID, ok := lineOf(b.Display, routes)
		lat, errLat := strconv.ParseFloat(b.Lat, 64)
		lon, errLon := strconv.ParseFloat(b.Lon, 64)
		if b.State != "on" || !ok || errLat != nil || errLon != nil {
			continue
		}
		out = append(out, follow.Bus{ID: plate, Label: plate, Lat: lat, Lon: lon, At: at, Lines: []follow.Line{{RouteID: routeID}}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// The line a display names: "  1" is 1; one naming no line, as "2B" on line 2's buses, is the longest line it begins with.
func lineOf(display string, routes map[string]int) (int, bool) {
	name := strings.Join(strings.Fields(display), "")
	if id, ok := routes[name]; ok {
		return id, true
	}
	best, id := "", 0
	for line, routeID := range routes {
		if len(line) > len(best) && strings.HasPrefix(name, line) {
			best, id = line, routeID
		}
	}
	return id, best != ""
}
