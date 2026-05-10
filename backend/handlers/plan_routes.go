package handlers

import (
	"bytes"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

var (
	otpReady   bool
	otpRunning bool
	otpCmd     *exec.Cmd
	otpMu      sync.RWMutex
	otpMaxMem  string = "4G"
)

// SetOTPMaxMemory configures the maximum memory for the OTP server.
func SetOTPMaxMemory(mx string) {
	otpMu.Lock()
	defer otpMu.Unlock()
	otpMaxMem = mx
}

// otpBaseURL returns the base URL of the local OTP server.
func otpBaseURL() string {
	if u := os.Getenv("OTP_URL"); u != "" {
		return u
	}
	return "http://localhost:18080"
}

const (
	// otpTimeout is the maximum time an OTP request is allowed to take.
	otpTimeout = 30 * time.Second
	// otpReadyRequestWait is how long a route-planning request can wait for a
	// server that is already starting up.
	otpReadyRequestWait = 45 * time.Second
)

// OTP GraphQL API response types

type otpGraphQLResponse struct {
	Data   *otpGraphQLData   `json:"data,omitzero"`
	Errors []otpGraphQLError `json:"errors,omitzero"`
}

type otpGraphQLError struct {
	Message string `json:"message"`
}

type otpGraphQLData struct {
	Plan *otpPlan `json:"plan"`
}

type otpPlan struct {
	Itineraries []otpItinerary `json:"itineraries"`
}

type otpItinerary struct {
	Duration          int64    `json:"duration"`
	WalkTime          int64    `json:"walkTime"`
	WalkDistance      float64  `json:"walkDistance"`
	GeneralizedCost   int64    `json:"generalizedCost"`
	NumberOfTransfers int      `json:"numberOfTransfers"`
	StartTime         int64    `json:"startTime"`
	EndTime           int64    `json:"endTime"`
	Legs              []otpLeg `json:"legs"`
}

type otpLeg struct {
	Mode              string          `json:"mode"`
	Duration          float64         `json:"duration"`
	Distance          float64         `json:"distance"`
	From              otpPlace        `json:"from"`
	To                otpPlace        `json:"to"`
	Route             *otpRoute       `json:"route,omitzero"`
	IntermediateStops []otpInterStop  `json:"intermediateStops,omitzero"`
	LegGeometry       *otpEncodedPoly `json:"legGeometry,omitzero"`
}

// otpInterStop represents a Stop object returned by intermediateStops
// (different from otpPlace which is used for from/to).
type otpInterStop struct {
	GtfsID string  `json:"gtfsId"`
	Name   string  `json:"name"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

type otpRoute struct {
	GtfsID    string `json:"gtfsId"`
	ShortName string `json:"shortName"`
	LongName  string `json:"longName"`
}

type otpPlace struct {
	Name string   `json:"name"`
	Lat  float64  `json:"lat"`
	Lon  float64  `json:"lon"`
	Stop *otpStop `json:"stop,omitzero"`
}

type otpStop struct {
	GtfsID string `json:"gtfsId"`
}

type otpEncodedPoly struct {
	Points string `json:"points"`
	Length int    `json:"length"`
}

// otpPlanQuery is the GraphQL query sent to the OTP GTFS API.
// searchWindow is set to 6h so that late-night "arrive by" queries
// (e.g. "last bus today" with a 03:00 cutoff) don't get pruned by
// OTP's dynamicSearchWindow when the deadline is far from any transit.
const otpPlanQuery = `{
  plan(
    from: { lat: %f, lon: %f }
    to:   { lat: %f, lon: %f }
    date: "%s"
    time: "%s"
    arriveBy: %t
    numItineraries: 12
    searchWindow: 21600
    transportModes: [{ mode: WALK }, { mode: TRANSIT }]
  ) {
    itineraries {
      duration
      walkTime
      walkDistance
      generalizedCost
      numberOfTransfers
      startTime
      endTime
      legs {
        mode
        duration
        distance
        from {
          name
          lat
          lon
          stop { gtfsId }
        }
        to {
          name
          lat
          lon
          stop { gtfsId }
        }
        route {
          gtfsId
          shortName
          longName
        }
        intermediateStops {
          name
          lat
          lon
          gtfsId
        }
        legGeometry { points length }
      }
    }
  }
}`

// otpWalkOnlyPlanQuery asks OTP for a walking-only itinerary so the frontend
// can always show a walk alternative.
const otpWalkOnlyPlanQuery = `{
  plan(
    from: { lat: %f, lon: %f }
    to:   { lat: %f, lon: %f }
    date: "%s"
    time: "%s"
    arriveBy: %t
    numItineraries: 1
    transportModes: [{ mode: WALK }]
  ) {
    itineraries {
      duration
      walkTime
      walkDistance
      generalizedCost
      numberOfTransfers
      startTime
      endTime
      legs {
        mode
        duration
        distance
        from {
          name
          lat
          lon
          stop { gtfsId }
        }
        to {
          name
          lat
          lon
          stop { gtfsId }
        }
        route {
          gtfsId
          shortName
          longName
        }
        intermediateStops {
          name
          lat
          lon
          gtfsId
        }
        legGeometry { points length }
      }
    }
  }
}`

func callOTPWithQuery(queryTpl string, fromLat, fromLng, toLat, toLng float64, when time.Time, arriveBy bool) (*otpPlan, error) {
	base := otpBaseURL()
	query := fmt.Sprintf(queryTpl, fromLat, fromLng, toLat, toLng,
		when.Format("2006-01-02"), when.Format("15:04"), arriveBy)

	payload, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, fmt.Errorf("otp: failed to marshal query: %w", err)
	}

	client := &http.Client{Timeout: otpTimeout}
	resp, err := client.Post(base+"/otp/gtfs/v1", "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("otp: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("otp: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("otp: server returned %d: %s", resp.StatusCode, string(body))
	}

	var gqlResp otpGraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("otp: failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("otp: graphql error: %s", gqlResp.Errors[0].Message)
	}

	if gqlResp.Data == nil || gqlResp.Data.Plan == nil {
		return nil, fmt.Errorf("otp: empty plan response")
	}

	return gqlResp.Data.Plan, nil
}

// callOTP makes a mixed transit+walk trip planning request.
func callOTP(fromLat, fromLng, toLat, toLng float64, when time.Time, arriveBy bool) (*otpPlan, error) {
	return callOTPWithQuery(otpPlanQuery, fromLat, fromLng, toLat, toLng, when, arriveBy)
}

// callOTPWalkOnly makes a walking-only request used as a guaranteed fallback.
func callOTPWalkOnly(fromLat, fromLng, toLat, toLng float64, when time.Time, arriveBy bool) (*otpPlan, error) {
	return callOTPWithQuery(otpWalkOnlyPlanQuery, fromLat, fromLng, toLat, toLng, when, arriveBy)
}

type plannerUnavailableError struct {
	err error
}

func (e plannerUnavailableError) Error() string {
	return e.err.Error()
}

func (e plannerUnavailableError) Unwrap() error {
	return e.err
}

func isOTPReady() bool {
	otpMu.RLock()
	defer otpMu.RUnlock()
	return otpReady
}

func waitForOTPReady(timeout time.Duration) bool {
	if isOTPReady() {
		return true
	}

	InitOTP()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline.C:
			return isOTPReady()
		case <-ticker.C:
			if isOTPReady() {
				return true
			}
		}
	}
}

// InitOTP checks that the OTP server is reachable and marks the planner as ready.
// Safe to call from a goroutine — can be called multiple times to re-check.
func InitOTP() {
	otpMu.Lock()
	if otpRunning {
		otpMu.Unlock()
		return
	}
	otpRunning = true
	otpReady = false
	otpMu.Unlock()

	go func() {
		defer func() {
			otpMu.Lock()
			otpRunning = false
			otpMu.Unlock()
		}()

		start := time.Now()
		base := otpBaseURL()

		// Poll OTP until it's ready (it may still be building the graph).
		maxWait := 10 * time.Minute
		poll := 5 * time.Second
		deadline := time.Now().Add(maxWait)

		log.Printf("otp: waiting for OTP server at %s …", base)
		for time.Now().Before(deadline) {
			client := &http.Client{Timeout: 5 * time.Second}
			payload := []byte(`{"query":"{__typename}"}`)
			resp, err := client.Post(base+"/otp/gtfs/v1", "application/json", bytes.NewReader(payload))
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					otpMu.Lock()
					otpReady = true
					otpMu.Unlock()
					log.Printf("otp: server ready in %s", time.Since(start).Round(time.Millisecond))
					return
				}
			}
			time.Sleep(poll)
		}
		log.Printf("otp: server did not become ready within %s", maxWait)
	}()
}

// stopOTPServer attempts to find and kill the process listening on port 8080.
func stopOTPServer() {
	otpMu.Lock()
	if otpCmd != nil && otpCmd.Process != nil {
		log.Printf("otp: stopping tracked process %d …", otpCmd.Process.Pid)
		_ = otpCmd.Process.Kill()
		_ = otpCmd.Wait()
		otpCmd = nil
	}
	otpMu.Unlock()

	log.Printf("otp: stopping local server on port 18080 (fallback check) …")

	if runtime.GOOS == "windows" {
		// On Windows, find the PID of the process listening on port 18080.
		cmd := exec.Command("cmd", "/c", "netstat -ano | findstr :18080")
		out, _ := cmd.Output()
		if len(out) == 0 {
			log.Printf("otp: no process found on port 18080")
			return
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}
			// Example: TCP    0.0.0.0:18080           0.0.0.0:0              LISTENING       1234
			if !strings.Contains(fields[1], ":18080") || fields[3] != "LISTENING" {
				continue
			}
			pid := fields[4]
			log.Printf("otp: killing process %s …", pid)
			killCmd := exec.Command("taskkill", "/F", "/PID", pid)
			if err := killCmd.Run(); err != nil {
				log.Printf("otp: failed to kill process %s: %v", pid, err)
			}
		}
	} else {
		// On Linux/macOS, find the PID of the process listening on port 8080 using fuser or lsof.
		// fuser -k 8080/tcp is a common way, but let's try to be more surgical.
		cmd := exec.Command("lsof", "-t", "-i:18080")
		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			log.Printf("otp: no process found on port 18080 (or lsof failed)")
			return
		}

		pids := strings.Fields(string(out))
		for _, pid := range pids {
			log.Printf("otp: killing process %s …", pid)
			killCmd := exec.Command("kill", "-9", pid)
			if err := killCmd.Run(); err != nil {
				log.Printf("otp: failed to kill process %s: %v", pid, err)
			}
		}
	}
}

// startOTPServer launches a new instance of the OTP server.
func startOTPServer() {
	jarPath := filepath.Join("services", "otp", "otp.jar")
	dataDir := filepath.Join("services", "otp", "cluj")

	log.Printf("otp: starting server with %s …", jarPath)
	// We use the same parameters as in start-otp.sh
	otpMu.RLock()
	mx := otpMaxMem
	otpMu.RUnlock()
	cmd := exec.Command("java", "-Xmx"+mx, "-XX:+UseZGC", "-jar", jarPath, "--build", "--serve", "--port", "18080", dataDir)

	if err := cmd.Start(); err != nil {
		log.Printf("otp: failed to start server: %v", err)
	} else {
		log.Printf("otp: server started with PID %d", cmd.Process.Pid)
		otpMu.Lock()
		otpCmd = cmd
		otpMu.Unlock()
	}
}

// CleanupOTP ensures the OTP process is killed.
func CleanupOTP() {
	otpMu.Lock()
	defer otpMu.Unlock()
	if otpCmd != nil && otpCmd.Process != nil {
		log.Printf("otp: cleaning up process %d …", otpCmd.Process.Pid)
		_ = otpCmd.Process.Kill()
		otpCmd = nil
	}
}

// TriggerOTPRebuild restarts the local OTP server to pick up new data
// from the latest GTFS and OSM data on disk.
func TriggerOTPRebuild() {
	base := otpBaseURL()
	// Only attempt restart if it's local.
	if !strings.Contains(base, "localhost") && !strings.Contains(base, "127.0.0.1") {
		log.Printf("otp: server is remote (%s), skipping local restart", base)
		return
	}

	stopOTPServer()

	otpMu.Lock()
	otpReady = false
	otpMu.Unlock()

	// Give a small delay for port to be released
	time.Sleep(2 * time.Second)

	startOTPServer()

	// Re-initialize (wait for it to become ready again)
	InitOTP()
}

type planLegResp struct {
	RouteID             int     `json:"route_id"`
	TripID              string  `json:"trip_id"`
	StartStopID         int     `json:"start_stop_id"`
	DestStopID          int     `json:"dest_stop_id"`
	RideSeconds         float64 `json:"ride_seconds"`
	IntermediateStopIDs []int   `json:"intermediate_stop_ids,omitzero"`
}

type planWalkSegment struct {
	Geometry    string  `json:"geometry"`
	DistanceM   float64 `json:"distance_m"`
	DurationSec float64 `json:"duration_sec"`
}

type planRouteResp struct {
	Legs               []planLegResp     `json:"legs"`
	IsDirect           bool              `json:"is_direct"`
	WalkStartMeters    float64           `json:"walk_start_meters"`
	WalkEndMeters      float64           `json:"walk_end_meters"`
	WalkTransferMeters float64           `json:"walk_transfer_meters"`
	TransitDurationSec float64           `json:"transit_duration_sec"`
	TotalDistance      float64           `json:"total_distance"`
	GeneralizedCost    int64             `json:"generalized_cost"`
	NumberOfTransfers  int               `json:"number_of_transfers"`
	StartTimeMs        int64             `json:"start_time_ms"`
	EndTimeMs          int64             `json:"end_time_ms"`
	WalkSegments       []planWalkSegment `json:"walk_segments,omitzero"`
}

type planResp struct {
	Plans  []planRouteResp        `json:"plans"`
	Stops  map[string]models.Stop `json:"stops"`
	Shapes map[string]shapeSlim   `json:"shapes"`
}

type shapeSlim struct {
	RouteShortName string            `json:"route_short_name"`
	RouteLongName  string            `json:"route_long_name"`
	RouteID        int               `json:"route_id"`
	RouteType      models.RouteType  `json:"route_type"`
	RouteColor     string            `json:"route_color"`
	StopTime       []models.StopTime `json:"stop_time,omitzero"`
	Timetable      *models.Timetable `json:"timetable,omitzero"`
}

func handlePlanRoutes(c fiber.Ctx, tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) error {
	if !waitForOTPReady(otpReadyRequestWait) {
		c.Set("Retry-After", "10")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":               "planner still starting",
			"retry_after_seconds": 10,
		})
	}

	fromLat, fromLng, toLat, toLng, err := parsePlanCoords(c)
	if err != nil {
		return err
	}

	when, arriveBy, err := parsePlanTime(c, tranzyClient.Location())
	if err != nil {
		return err
	}

	reqTime := when
	if reqTime.IsZero() {
		reqTime = time.Now().In(tranzyClient.Location())
	}

	plan, err := callOTP(fromLat, fromLng, toLat, toLng, reqTime, arriveBy)
	if err != nil {
		log.Printf("plan_routes: failed: %v", err)
		var plannerErr plannerUnavailableError
		if errors.As(err, &plannerErr) {
			c.Set("Retry-After", "10")
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":               "planner unavailable",
				"retry_after_seconds": 10,
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "routing failed"})
	}

	itineraries := make([]otpItinerary, 0, len(plan.Itineraries)+1)
	itineraries = append(itineraries, plan.Itineraries...)

	// Try to force-include a walk-only alternative; if it fails, keep transit results.
	walkOnlyPlan, walkErr := callOTPWalkOnly(fromLat, fromLng, toLat, toLng, reqTime, arriveBy)
	if walkErr != nil {
		log.Printf("plan_routes: walk-only fallback failed: %v", walkErr)
	} else if walkOnlyPlan != nil && len(walkOnlyPlan.Itineraries) > 0 {
		itineraries = append(itineraries, walkOnlyPlan.Itineraries...)
	}

	if len(itineraries) == 0 {
		c.Set("Cache-Control", "public, max-age=300")
		return c.JSON(planResp{
			Plans:  []planRouteResp{},
			Stops:  map[string]models.Stop{},
			Shapes: map[string]shapeSlim{},
		})
	}

	resp, err := enrichOTPResponse(itineraries, tranzyClient, ctpCjClient, cacheTimes)
	if err != nil {
		log.Printf("plan_routes: enrich failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "routing failed"})
	}

	c.Set("Cache-Control", "public, max-age=300")
	return c.JSON(resp)
}

func parsePlanCoords(c fiber.Ctx) (fromLat, fromLng, toLat, toLng float64, err error) {
	fromLatS := c.Query("from_lat")
	fromLngS := c.Query("from_lng")
	toLatS := c.Query("to_lat")
	toLngS := c.Query("to_lng")
	if fromLatS == "" || fromLngS == "" || toLatS == "" || toLngS == "" {
		return 0, 0, 0, 0, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from_lat, from_lng, to_lat, to_lng are required"})
	}
	fromLat, err = strconv.ParseFloat(fromLatS, 64)
	if err != nil {
		return 0, 0, 0, 0, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from_lat"})
	}
	fromLng, err = strconv.ParseFloat(fromLngS, 64)
	if err != nil {
		return 0, 0, 0, 0, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from_lng"})
	}
	toLat, err = strconv.ParseFloat(toLatS, 64)
	if err != nil {
		return 0, 0, 0, 0, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to_lat"})
	}
	toLng, err = strconv.ParseFloat(toLngS, 64)
	if err != nil {
		return 0, 0, 0, 0, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to_lng"})
	}

	// Validate geographic bounds: reject NaN, Inf, and out-of-range coordinates.
	if !isValidLat(fromLat) || !isValidLng(fromLng) || !isValidLat(toLat) || !isValidLng(toLng) {
		return 0, 0, 0, 0, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "coordinates out of valid range"})
	}

	return fromLat, fromLng, toLat, toLng, nil
}

// parsePlanTime parses optional `time` and `arrive_by` query params.
// `time` may be RFC3339 ("2006-01-02T15:04:05Z07:00") or local "2006-01-02T15:04".
// When `time` is omitted or empty, returns a zero time meaning "now".
func parsePlanTime(c fiber.Ctx, loc *time.Location) (time.Time, bool, error) {
	timeStr := strings.TrimSpace(c.Query("time"))
	arriveBy := strings.EqualFold(strings.TrimSpace(c.Query("arrive_by")), "true") ||
		c.Query("arrive_by") == "1"

	if timeStr == "" {
		return time.Time{}, arriveBy, nil
	}

	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t.In(loc), arriveBy, nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", timeStr, loc); err == nil {
		return t, arriveBy, nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", timeStr, loc); err == nil {
		return t, arriveBy, nil
	}
	return time.Time{}, false, c.Status(fiber.StatusBadRequest).
		JSON(fiber.Map{"error": "invalid time (expected RFC3339 or 2006-01-02T15:04)"})
}

func isValidLat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -90 && v <= 90
}

func isValidLng(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -180 && v <= 180
}

// placeStopID extracts the gtfsId from an otpPlace's nested stop field.
func placeStopID(p otpPlace) string {
	if p.Stop != nil {
		return p.Stop.GtfsID
	}
	return ""
}

// extractOTPStopID extracts the numeric stop ID from an OTP feed-scoped ID
// like "1:12345" → 12345 or falls back to nearest-stop matching.
func extractOTPStopID(otpID string, lat, lon float64, allStops []models.Stop) models.Stop {
	if otpID != "" {
		// OTP stop IDs are typically "feedId:stopId"
		if _, after, ok := strings.Cut(otpID, ":"); ok {
			if id, err := strconv.Atoi(after); err == nil {
				for _, s := range allStops {
					if s.StopID == id {
						return s
					}
				}
			}
		}
		// Try parsing the whole thing as a number
		if id, err := strconv.Atoi(otpID); err == nil {
			for _, s := range allStops {
				if s.StopID == id {
					return s
				}
			}
		}
	}
	// Fall back to nearest stop by coordinates
	return nearestStop(allStops, lat, lon)
}

func enrichOTPResponse(itineraries []otpItinerary, tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) (*planResp, error) {
	if len(itineraries) == 0 {
		return &planResp{Plans: []planRouteResp{}, Stops: map[string]models.Stop{}, Shapes: map[string]shapeSlim{}}, nil
	}

	allStops, err := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, StopFilter{})
	if err != nil {
		return nil, err
	}
	allRoutes, err := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{})
	if err != nil {
		return nil, err
	}

	routeByName := make(map[string]models.Route, len(allRoutes))
	for _, r := range allRoutes {
		routeByName[strings.ToUpper(strings.TrimSpace(r.RouteShortName))] = r
	}

	stopsMap := make(map[string]models.Stop)
	shapesMap := make(map[string]shapeSlim)
	plans := make([]planRouteResp, 0)
	seenPlans := make(map[string]struct{})

	for _, itin := range itineraries {
		var curLegs []planLegResp
		var walkStart, walkEnd, walkTransfer float64
		var transitSec float64
		var walkSegments []planWalkSegment

		for i, leg := range itin.Legs {
			switch leg.Mode {
			case "WALK":
				meters := leg.Distance

				// Check if there is any transit leg later
				hasTripAfter := false
				for j := i + 1; j < len(itin.Legs); j++ {
					if itin.Legs[j].Mode != "WALK" {
						hasTripAfter = true
						break
					}
				}

				if len(curLegs) == 0 {
					walkStart += meters
				} else if !hasTripAfter {
					walkEnd += meters
				} else {
					walkTransfer += meters
				}

				if leg.LegGeometry != nil && leg.LegGeometry.Points != "" {
					walkSegments = append(walkSegments, planWalkSegment{
						Geometry:    leg.LegGeometry.Points,
						DistanceM:   meters,
						DurationSec: leg.Duration,
					})
				}

			case "BUS", "TRAM", "RAIL", "SUBWAY", "FERRY", "CABLE_CAR", "GONDOLA", "FUNICULAR", "TROLLEYBUS", "TRANSIT":
				if leg.Route == nil {
					continue
				}
				routeName := leg.Route.ShortName
				if routeName == "" {
					routeName = leg.Route.LongName
				}
				route, ok := routeByName[strings.ToUpper(strings.TrimSpace(routeName))]
				if !ok {
					continue
				}

				startStop := extractOTPStopID(placeStopID(leg.From), leg.From.Lat, leg.From.Lon, allStops)
				destStop := extractOTPStopID(placeStopID(leg.To), leg.To.Lat, leg.To.Lon, allStops)

				var intermediates []int
				for _, iStop := range leg.IntermediateStops {
					s := extractOTPStopID(iStop.GtfsID, iStop.Lat, iStop.Lon, allStops)
					if s.StopID != startStop.StopID && s.StopID != destStop.StopID {
						intermediates = append(intermediates, s.StopID)
					}
				}

				rideSec := leg.Duration
				transitSec += rideSec

				tripID := ""
				stopInfo, _ := GetStopInfo(tranzyClient, ctpCjClient, cacheTimes, StopFilter{StopID: &startStop.StopID})
				if stopInfo != nil {
					target0 := strconv.Itoa(route.RouteID) + "_0"
					target1 := strconv.Itoa(route.RouteID) + "_1"

					found0 := slices.Contains(stopInfo.OutgoingTripIds, target0)
					found1 := slices.Contains(stopInfo.IncomingTripIds, target1)

					if found0 && !found1 {
						tripID = target0
					} else if found1 && !found0 {
						tripID = target1
					} else if found0 && found1 {
						if destStopInfo, _ := GetStopInfo(tranzyClient, ctpCjClient, cacheTimes, StopFilter{StopID: &destStop.StopID}); destStopInfo != nil {
							dFound0 := slices.Contains(destStopInfo.OutgoingTripIds, target0)
							dFound1 := slices.Contains(destStopInfo.IncomingTripIds, target1)
							if dFound0 && !dFound1 {
								tripID = target0
							} else if dFound1 && !dFound0 {
								tripID = target1
							}
						}
					}
				}
				if tripID == "" {
					tripID = strconv.Itoa(route.RouteID) + "_0"
				}

				// Use the already-fetched stopInfo to add alternative routes that
				// cover the exact same stop sequence (start → all intermediates → dest)
				// in the same direction. Routes that only share the endpoints but take
				// a different path (e.g. a different route number) are excluded.
				if stopInfo != nil {
					suffix := "_0"
					if strings.HasSuffix(tripID, "_1") {
						suffix = "_1"
					}

					// Build the full ordered stop sequence the chosen leg must match.
					required := make([]int, 0, len(intermediates)+2)
					required = append(required, startStop.StopID)
					required = append(required, intermediates...)
					required = append(required, destStop.StopID)

					for _, si := range stopInfo.ShapesInfo {
						key := strconv.Itoa(si.RouteId)
						if _, exists := shapesMap[key]; exists {
							continue
						}
						altTripID := strconv.Itoa(si.RouteId) + suffix

						// Walk the stop-times in order and match every required stop
						// sequentially.  All required stops must appear in order.
						reqIdx := 0
						for _, st := range si.StopTimes {
							if st.TripID != altTripID {
								continue
							}
							if st.StopID == required[reqIdx] {
								reqIdx++
								if reqIdx == len(required) {
									break
								}
							}
						}
						if reqIdx < len(required) {
							// This route does not serve all stops in the correct order.
							continue
						}

						t := si.Timetable
						shapesMap[key] = shapeSlim{
							RouteShortName: si.RouteShortName,
							RouteLongName:  si.RouteLongName,
							RouteID:        si.RouteId,
							RouteType:      si.RouteType,
							RouteColor:     models.ResolveRouteDisplayColor(si.RouteShortName),
							StopTime:       si.StopTimes,
							Timetable:      &t,
						}
					}
				}

				curLegs = append(curLegs, planLegResp{
					RouteID:             route.RouteID,
					TripID:              NormalizeTripID(tripID),
					StartStopID:         startStop.StopID,
					DestStopID:          destStop.StopID,
					RideSeconds:         rideSec,
					IntermediateStopIDs: intermediates,
				})

				stopsMap[strconv.Itoa(startStop.StopID)] = startStop
				stopsMap[strconv.Itoa(destStop.StopID)] = destStop
				if _, exists := shapesMap[strconv.Itoa(route.RouteID)]; !exists {
					stopTimes, _ := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &route.RouteShortName})
					timetable, _ := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, route.RouteShortName)

					shapesMap[strconv.Itoa(route.RouteID)] = shapeSlim{
						RouteShortName: route.RouteShortName,
						RouteLongName:  route.RouteLongName,
						RouteID:        route.RouteID,
						RouteType:      route.RouteType,
						RouteColor:     models.ResolveRouteDisplayColor(route.RouteShortName),
						StopTime:       stopTimes,
						Timetable:      timetable,
					}
				}
			}
		}

		if len(curLegs) > 0 {
			// Build a structural key based on route/stop/direction — ignoring
			// ride duration so that the same logical journey discovered at
			// different departure times is properly deduplicated.
			var keyParts []string
			for _, l := range curLegs {
				keyParts = append(keyParts, fmt.Sprintf("%d:%s:%d>%d",
					l.RouteID, l.TripID, l.StartStopID, l.DestStopID))
			}
			planKey := strings.Join(keyParts, "|")
			if _, seen := seenPlans[planKey]; seen {
				continue
			}
			seenPlans[planKey] = struct{}{}

			totalDist := walkStart + walkEnd + walkTransfer
			for _, l := range curLegs {
				totalDist += float64(l.RideSeconds) * 8.3
			}
			plans = append(plans, planRouteResp{
				Legs:               curLegs,
				IsDirect:           len(curLegs) == 1,
				WalkStartMeters:    walkStart,
				WalkEndMeters:      walkEnd,
				WalkTransferMeters: walkTransfer,
				TransitDurationSec: transitSec,
				TotalDistance:      totalDist,
				GeneralizedCost:    itin.GeneralizedCost,
				NumberOfTransfers:  itin.NumberOfTransfers,
				StartTimeMs:        itin.StartTime,
				EndTimeMs:          itin.EndTime,
				WalkSegments:       walkSegments,
			})
		} else {
			walkOnlyMeters := walkStart + walkEnd + walkTransfer
			if walkOnlyMeters <= 0 {
				continue
			}
			planKey := fmt.Sprintf("walk:%d", int(math.Round(walkOnlyMeters)))
			if _, seen := seenPlans[planKey]; seen {
				continue
			}
			seenPlans[planKey] = struct{}{}

			plans = append(plans, planRouteResp{
				Legs:               []planLegResp{},
				IsDirect:           true,
				WalkStartMeters:    walkStart,
				WalkEndMeters:      walkEnd,
				WalkTransferMeters: walkTransfer,
				TransitDurationSec: 0,
				TotalDistance:      walkOnlyMeters,
				GeneralizedCost:    itin.GeneralizedCost,
				NumberOfTransfers:  itin.NumberOfTransfers,
				StartTimeMs:        itin.StartTime,
				EndTimeMs:          itin.EndTime,
				WalkSegments:       walkSegments,
			})
		}
	}

	if plans == nil {
		plans = []planRouteResp{}
	}

	return &planResp{
		Plans:  plans,
		Stops:  stopsMap,
		Shapes: shapesMap,
	}, nil
}

func nearestStop(stops []models.Stop, lat, lon float64) models.Stop {
	best := stops[0]
	bestDist := math.MaxFloat64
	for _, s := range stops {
		d := haversineMeters(lat, lon, s.StopLat, s.StopLon)
		if d < bestDist {
			bestDist = d
			best = s
		}
	}
	return best
}
