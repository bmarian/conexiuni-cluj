package suceava

import (
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/follow"
	"conexiuni-cluj/feeds/traces"
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The buses on harta.tpl-sv.ro, TPL's own live map: plate, position and the line on the bus's display, nothing more.
type realtime struct {
	url   string
	trace feeds.Tracing
	// Records the buses of the lines the pages add; nil in tests.
	rec      *traces.Recorder
	follower *follow.Follower
}

func newRealtime(url string, ids feeds.IDMap, trace feeds.Tracing) *realtime {
	return &realtime{url: url, trace: trace, follower: follow.New(ids, trace)}
}

func (s *realtime) Budget() feeds.Budget { return feeds.Budget{} }

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
	now := time.Now()
	if s.rec != nil {
		var seen []traces.Sighting
		for _, b := range buses {
			if line := tracedLine(b.label, s.rec.Traced); line != "" {
				seen = append(seen, traces.Sighting{Line: line, Vehicle: b.plate, Lat: b.lat, Lon: b.lon})
			}
		}
		if err := s.rec.Record(seen, now); err != nil {
			log.Printf("suceava: line traces: %v", err)
		}
	}
	return s.vehicles(buses, now)
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
	routes := s.trace.RouteIDs()
	out := make([]follow.Bus, 0, len(buses))
	for _, b := range buses {
		out = append(out, follow.Bus{ID: b.plate, Label: b.plate, Lat: b.lat, Lon: b.lon, Lines: lineCandidates(b.label, routes)})
	}
	return s.follower.Vehicles(out, now)
}

// The line a display's number names ("M105." is M105, "  5" is 5), then, when the text fills the display unpadded and may be cut short ("M10" on a bus of M100), the lines it begins.
func lineCandidates(label string, routes map[string]int) []follow.Line {
	number, ok := displayNumber(label)
	if !ok {
		return nil
	}
	var out []follow.Line
	for _, short := range namesOf(number) {
		if id, ok := routes[short]; ok {
			out = append(out, follow.Line{RouteID: id})
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
			out = append(out, follow.Line{RouteID: routes[short], Cut: true})
		}
	}
	return out
}

// The number on a display, letters and punctuation dropped.
func displayNumber(label string) (string, bool) {
	number := strings.TrimLeftFunc(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, label), unicode.IsLetter)
	n, err := strconv.Atoi(number)
	return number, err == nil && n > 0
}

// A number is a line of the city or a metropolitan one.
func namesOf(number string) []string { return []string{number, "M" + number} }

// The traced line a display names; a cut text names none, as the line it was cut from is not known.
func tracedLine(label string, traced func(string) bool) string {
	number, ok := displayNumber(label)
	if !ok {
		return ""
	}
	for _, name := range namesOf(number) {
		if traced(name) {
			return name
		}
	}
	return ""
}
