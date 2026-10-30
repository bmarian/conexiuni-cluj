package tranzy

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Requests per Snapshot.
const snapshotCalls = 5

type Source struct {
	client *Client
}

func New(client *Client) *Source { return &Source{client: client} }

func (s *Source) Credit() feeds.Credit {
	return feeds.Credit{Name: "tranzy.ai", URL: "https://tranzy.ai/"}
}

func (s *Source) Snapshot() (*feeds.Snapshot, error) {
	if left, resetAt := s.client.defaultQuota.remainingUntil(); left < snapshotCalls {
		return nil, &QuotaError{Left: left, Needed: snapshotCalls, ResetAt: resetAt}
	}
	routes, err := fetch[[]models.Route](s.client, "/routes")
	if err != nil {
		return nil, err
	}
	for i := range routes {
		routes[i].RouteColor = lineColor(routes[i])
	}
	stops, err := fetch[[]models.Stop](s.client, "/stops")
	if err != nil {
		return nil, err
	}
	trips, err := fetch[[]models.Trip](s.client, "/trips")
	if err != nil {
		return nil, err
	}
	shapes, err := fetch[[]models.Shape](s.client, "/shapes")
	if err != nil {
		return nil, err
	}
	tripStops, err := fetch[[]models.TripStop](s.client, "/stop_times")
	if err != nil {
		return nil, err
	}
	for i := range trips {
		trips[i].TripID = PatternID(trips[i].TripID)
		trips[i].ShapeID = PatternID(trips[i].ShapeID)
	}
	for i := range shapes {
		shapes[i].ShapeID = PatternID(shapes[i].ShapeID)
	}
	// Tranzy returns shape points in rotated blocks, not by sequence.
	sort.Slice(shapes, func(i, j int) bool {
		if shapes[i].ShapeID != shapes[j].ShapeID {
			return shapes[i].ShapeID < shapes[j].ShapeID
		}
		return shapes[i].ShapePtSequence < shapes[j].ShapePtSequence
	})
	for i := range tripStops {
		tripStops[i].TripID = PatternID(tripStops[i].TripID)
	}
	return &feeds.Snapshot{Routes: routes, Stops: stops, Trips: trips, Shapes: shapes, TripStops: tripStops}, nil
}

// A night line has an N before or after its number, as STB's N1 or CTP's 25N and M26N.
var nightLine = regexp.MustCompile(`(?i)^N\d|\dN$`)

// Tranzy's own colours don't tell its lines apart, so each kind takes STB's colour for it.
func lineColor(r models.Route) string {
	switch {
	case nightLine.MatchString(r.RouteShortName):
		return "#2D2E83"
	case r.RouteType == models.Tram || r.RouteType == models.CableTram:
		return "#BE1622"
	case r.RouteType == models.Trolleybus:
		return "#008D36"
	}
	return "#1D71B8"
}

func (s *Source) Cadence() time.Duration {
	perDay := s.client.defaultQuota.limit / snapshotCalls
	if perDay <= 0 {
		return 24 * time.Hour
	}
	return 24 * time.Hour / time.Duration(perDay)
}

func (s *Source) Vehicles() ([]models.Vehicle, error) {
	vehicles, err := fetch[[]models.Vehicle](s.client, "/vehicles")
	if err != nil {
		return nil, err
	}
	out := make([]models.Vehicle, 0, len(vehicles))
	for _, v := range vehicles {
		if v.RouteID == -1 || v.TripID == "-1" {
			continue
		}
		v.TripID = PatternID(v.TripID)
		out = append(out, v)
	}
	return out, nil
}

func (s *Source) Budget() feeds.Budget {
	return feeds.Budget{Remaining: s.client.vehiclesQuota.remaining(), Limit: s.client.vehiclesQuota.limit}
}

func (s *Source) Quotas() []feeds.Quota {
	out := make([]feeds.Quota, 0, 2)
	for _, q := range []*dailyQuota{s.client.vehiclesQuota, s.client.defaultQuota} {
		out = append(out, feeds.Quota{Name: q.name, Used: q.limit - q.remaining(), Limit: q.limit})
	}
	return out
}

// PatternID keeps the "{route}_{direction}" of a Tranzy trip or shape id.
func PatternID(id string) string {
	parts := strings.Split(id, "_")
	if len(parts) >= 2 {
		return parts[0] + "_" + parts[1]
	}
	return id
}

func fetch[T any](client *Client, endpoint string) (T, error) {
	var result T
	data, err := client.DoRequest(endpoint, nil)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("tranzy: %s: %w", endpoint, err)
	}
	return result, nil
}
