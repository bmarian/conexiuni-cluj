package suceava

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/live"
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// The buses on harta.tpl-sv.ro, TPL's own live map: plate, position and the line on the bus's display, nothing more.
type realtime struct {
	url   string
	ids   feeds.IDMap
	trace feeds.Tracing

	mu   sync.Mutex
	last map[string]reading
}

func newRealtime(url string, ids feeds.IDMap, trace feeds.Tracing) *realtime {
	return &realtime{url: url, ids: ids, trace: trace, last: map[string]reading{}}
}

func (s *realtime) Budget() feeds.Budget { return feeds.Budget{} }

// A bus as the map shows it; its pattern once its movement tells.
type reading struct {
	lat, lon float64
	at       int64
	// Where it last moved from: a bus creeping a few metres a poll has still moved once they add up.
	fromLat, fromLon   float64
	routeID, direction int
	placed             bool
}

type sighting struct {
	plate    string
	lat, lon float64
	label    string
}

func (s *realtime) Vehicles() ([]models.Vehicle, error) {
	data, err := feeds.Download(s.url)
	if err != nil {
		return nil, fmt.Errorf("harta.tpl-sv.ro: %w", err)
	}
	buses, err := parseBuses(data)
	if err != nil {
		return nil, fmt.Errorf("harta.tpl-sv.ro: %w", err)
	}
	return s.vehicles(buses, time.Now())
}

// Two fleets, each a JSON object inside a string, sometimes in parentheses or a one-element array.
func parseBuses(data []byte) ([]sighting, error) {
	var a struct {
		Thoreb string `json:"thoreb"`
		Karsan string `json:"karsan"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	var thoreb map[string]struct {
		Contact string `json:"1"`
		Lat     string `json:"2"`
		Lon     string `json:"3"`
		Line    string `json:"4"`
	}
	var karsan map[string]struct {
		Contact string      `json:"contact"`
		Lat     json.Number `json:"lat"`
		Lon     json.Number `json:"lon"`
		Journey string      `json:"journey"`
		Plate   string      `json:"plate_no"`
	}
	if err := fleet(a.Thoreb, &thoreb); err != nil {
		return nil, fmt.Errorf("thoreb: %w", err)
	}
	if err := fleet(a.Karsan, &karsan); err != nil {
		return nil, fmt.Errorf("karsan: %w", err)
	}
	var out []sighting
	add := func(plate, contact, lat, lon, label string) {
		la, errLa := strconv.ParseFloat(strings.TrimSpace(lat), 64)
		lo, errLo := strconv.ParseFloat(strings.TrimSpace(lon), 64)
		if contact == "on" && errLa == nil && errLo == nil && la != 0 && lo != 0 {
			out = append(out, sighting{plate: plate, lat: la, lon: lo, label: label})
		}
	}
	for plate, b := range thoreb {
		add(plate, b.Contact, b.Lat, b.Lon, b.Line)
	}
	for id, b := range karsan {
		plate := b.Plate
		if plate == "" {
			plate = strings.ToUpper(strings.ReplaceAll(id, "_", " "))
		}
		add(plate, b.Contact, b.Lat.String(), b.Lon.String(), b.Journey)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].plate < out[j].plate })
	return out, nil
}

func fleet(raw string, into any) error {
	b := bytes.TrimSpace([]byte(raw))
	if len(b) == 0 {
		return nil
	}
	if b[0] == '(' && b[len(b)-1] == ')' {
		b = bytes.TrimSpace(b[1 : len(b)-1])
	}
	if len(b) > 0 && b[0] == '[' {
		var list []json.RawMessage
		if err := json.Unmarshal(b, &list); err != nil || len(list) == 0 {
			return err
		}
		b = list[0]
	}
	return json.Unmarshal(b, into)
}

func (s *realtime) vehicles(buses []sighting, now time.Time) ([]models.Vehicle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	routes := s.trace.RouteIDs()
	seen := make(map[string]reading, len(buses))
	var out []models.Vehicle
	for _, b := range buses {
		lines := lineCandidates(b.label, routes)
		if len(lines) == 0 {
			continue
		}
		prev, hasPrev := s.last[b.plate]
		cur := reading{lat: b.lat, lon: b.lon, at: now.Unix(), fromLat: b.lat, fromLon: b.lon}
		var moved bool
		cur.routeID, cur.direction, cur.placed, moved = s.follow(lines, prev, hasPrev, cur)
		if hasPrev && !moved {
			cur.fromLat, cur.fromLon = prev.fromLat, prev.fromLon
		}
		seen[b.plate] = cur
		if !cur.placed {
			continue
		}
		id, err := s.ids.ExternalID("vehicle", b.plate)
		if err != nil {
			return nil, err
		}
		speed := 0.0
		if hasPrev && cur.at > prev.at {
			speed = predictions.HaversineMeters(prev.lat, prev.lon, cur.lat, cur.lon) / float64(cur.at-prev.at) * 3.6
		}
		out = append(out, models.Vehicle{
			ID: id, Label: b.plate, Latitude: b.lat, Longitude: b.lon, Timestamp: now.UTC().Format(time.RFC3339),
			Speed: speed, RawSpeed: speed, RouteID: cur.routeID, TripID: models.PatternID(cur.routeID, cur.direction),
		})
	}
	s.last = seen
	return out, nil
}

type lineCandidate struct {
	routeID int
	// A longer number the label may have been cut from.
	cut bool
}

// The line a display's number names ("M105." is M105, "  5" is 5), then, when the text fills the display unpadded and may be cut short ("M10" on a bus of M100), the lines it begins.
func lineCandidates(label string, routes map[string]int) []lineCandidate {
	name := strings.ToUpper(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, label))
	number := strings.TrimLeftFunc(name, unicode.IsLetter)
	if n, err := strconv.Atoi(number); err != nil || n <= 0 {
		return nil
	}
	var out []lineCandidate
	for _, short := range []string{number, "M" + number} {
		if id, ok := routes[short]; ok {
			out = append(out, lineCandidate{routeID: id})
		}
	}
	if strings.TrimSpace(label) != label {
		return out
	}
	shorts := make([]string, 0, len(routes))
	for short := range routes {
		shorts = append(shorts, short)
	}
	sort.Strings(shorts)
	for _, short := range shorts {
		digits := strings.TrimLeftFunc(short, unicode.IsLetter)
		if len(digits) > len(number) && strings.HasPrefix(digits, number) && strings.IndexFunc(digits, func(r rune) bool { return !unicode.IsDigit(r) }) < 0 {
			out = append(out, lineCandidate{routeID: routes[short], cut: true})
		}
	}
	return out
}

type fit struct {
	routeID, direction int
	cut                bool
	off, progress      float64
	moving             bool
}

// follow keeps a bus on its pattern while it fits and does not go back along it, else takes the pattern it moves forward on, or the only one that fits it.
func (s *realtime) follow(lines []lineCandidate, prev reading, hasPrev bool, cur reading) (routeID, direction int, placed, moved bool) {
	moved = hasPrev && predictions.HaversineMeters(prev.fromLat, prev.fromLon, cur.lat, cur.lon) > live.StationaryRadiusMeters
	var fits []fit
	labelled := false
	for _, line := range lines {
		for direction := 0; direction <= 1; direction++ {
			along, off, ok := s.trace.Along(line.routeID, direction, cur.lat, cur.lon)
			if !ok || off > predictions.MaxOffRouteMeters {
				continue
			}
			f := fit{routeID: line.routeID, direction: direction, cut: line.cut, off: off}
			if moved {
				if before, prevOff, ok := s.trace.Along(line.routeID, direction, prev.fromLat, prev.fromLon); ok && prevOff <= predictions.MaxOffRouteMeters {
					f.progress, f.moving = along-before, true
				}
			}
			fits = append(fits, f)
			labelled = labelled || !line.cut
		}
	}
	if labelled {
		// The line on the display wins whenever it fits.
		fits = slices.DeleteFunc(fits, func(f fit) bool { return f.cut })
	}
	if hasPrev && prev.placed {
		for _, f := range fits {
			if f.routeID == prev.routeID && f.direction == prev.direction && (!f.moving || f.progress >= 0) {
				return f.routeID, f.direction, true, moved
			}
		}
	}
	var best *fit
	for i, f := range fits {
		if f.moving && f.progress > 0 && (best == nil || f.off < best.off) {
			best = &fits[i]
		}
	}
	if best != nil {
		return best.routeID, best.direction, true, moved
	}
	if len(fits) == 1 {
		return fits[0].routeID, fits[0].direction, true, moved
	}
	return 0, 0, false, moved
}
