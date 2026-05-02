package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
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

const clujFeedID = 2121

var (
	mobrouteOnce  sync.Once
	mobrouteReady bool
	mobrouteMu    sync.RWMutex
)

// mobrouteBin returns the absolute path to the mobroute CLI binary
// located in services/mobroute/ relative to the working directory
// (expected to be the backend/ directory).
func mobrouteBin() string {
	// The binary is always the Linux 'mobroute' file, even on Windows
	// where we run it through WSL.
	return filepath.Join("services", "mobroute", "mobroute")
}

// mobrouteTimeout is the maximum time a single mobroute CLI invocation
// is allowed to run before being killed.
const mobrouteTimeout = 2 * time.Minute

// allowedSubcmds is the strict allowlist of mobroute subcommands we invoke.
var allowedSubcmds = map[string]struct{}{
	"database": {},
	"route":    {},
}

// runMobroute executes the mobroute CLI with the given subcommand and
// JSON params string. Returns stdout (the JSON result). Logs from the
// CLI go to stderr and are forwarded to the Go logger.
func runMobroute(subcmd, paramsJSON string) ([]byte, error) {
	// Validate subcommand against allowlist to prevent arbitrary CLI usage.
	if _, ok := allowedSubcmds[subcmd]; !ok {
		return nil, fmt.Errorf("mobroute: disallowed subcommand %q", subcmd)
	}

	bin := mobrouteBin()

	// Verify the binary exists and resolve to absolute path to prevent
	// PATH-based hijacking.
	absBin, err := filepath.Abs(bin)
	if err != nil {
		return nil, fmt.Errorf("mobroute: cannot resolve binary path: %w", err)
	}
	if _, err := os.Stat(absBin); err != nil {
		return nil, fmt.Errorf("mobroute: binary not found at %s: %w", absBin, err)
	}

	// Use a timeout context to prevent runaway processes.
	ctx, cancel := context.WithTimeout(context.Background(), mobrouteTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		absDir := filepath.Dir(absBin)
		cmd = exec.CommandContext(ctx, "wsl", "--cd", absDir, "--exec", "./mobroute", subcmd, "-p", paramsJSON)
	} else {
		cmd = exec.CommandContext(ctx, absBin, subcmd, "-p", paramsJSON)
	}

	// Build a minimal environment — only HOME is needed by the CLI.
	// This avoids leaking secrets (API keys, tokens) to the subprocess.
	cmd.Env = []string{"HOME=" + homeDir()}
	if runtime.GOOS == "windows" {
		// Translate HOME path for WSL.
		cmd.Env = append(cmd.Env, "WSLENV=HOME/p")
	}
	if p := os.Getenv("PATH"); p != "" {
		cmd.Env = append(cmd.Env, "PATH="+p)
	}
	if runtime.GOOS == "windows" {
		if sd := os.Getenv("SYSTEMDRIVE"); sd != "" {
			cmd.Env = append(cmd.Env, "SYSTEMDRIVE="+sd)
		}
		if sr := os.Getenv("SYSTEMROOT"); sr != "" {
			cmd.Env = append(cmd.Env, "SYSTEMROOT="+sr)
		}
		if tmp := os.Getenv("TMP"); tmp != "" {
			cmd.Env = append(cmd.Env, "TMP="+tmp)
		}
	}

	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("mobroute %s failed: %w\nstderr: %s", subcmd, err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("mobroute %s failed: %w", subcmd, err)
	}
	return out, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}

// InitMobroute loads the Cluj GTFS feed and computes derived tables
// via the mobroute CLI. Safe to call from a goroutine — runs once.
func InitMobroute() {
	mobrouteOnce.Do(func() {
		start := time.Now()

		bin := mobrouteBin()
		if _, err := os.Stat(bin); err != nil {
			log.Printf("mobroute: binary not found at %s: %v", bin, err)
			return
		}

		// 1. Load GTFS from Mobility Database (feed 2121).
		loadParams := fmt.Sprintf(`{"op":"loadmdbgtfs","feed_ids":[%d],"loadmdbgtfs_cacheclear":false}`, clujFeedID)
		log.Printf("mobroute: loading GTFS feed %d …", clujFeedID)
		if _, err := runMobroute("database", loadParams); err != nil {
			log.Printf("mobroute: loadmdbgtfs failed: %v", err)
			return
		}
		log.Printf("mobroute: GTFS feed loaded")

		// 2. Compute routing-optimised derived tables.
		computeParams := fmt.Sprintf(`{"op":"compute","feed_ids":[%d]}`, clujFeedID)
		log.Printf("mobroute: computing derived tables …")
		if _, err := runMobroute("database", computeParams); err != nil {
			log.Printf("mobroute: compute failed: %v", err)
			return
		}

		mobrouteMu.Lock()
		mobrouteReady = true
		mobrouteMu.Unlock()

		log.Printf("mobroute: runtime ready in %s", time.Since(start).Round(time.Millisecond))
	})
}

type cliRouteResponse struct {
	Legs        []cliRouteLeg  `json:"legs"`
	Connections []cliRouteConn `json:"connections,omitzero"`
}

type cliRouteLeg struct {
	LegType       string     `json:"leg_type"`
	LegDuration   string     `json:"leg_duration"`
	LegBeginTime  time.Time  `json:"leg_begin_time"`
	LegFromCoords [2]float64 `json:"leg_from_coords"`
	LegToCoords   [2]float64 `json:"leg_to_coords"`

	// trip fields
	TripRoute *string            `json:"trip_route,omitzero"`
	TripID    *string            `json:"trip_id,omitzero"`
	TripStops *[]*cliStopDetails `json:"trip_stops,omitzero"`

	// walk fields
	WalkDistKm *float64 `json:"walk_dist_km,omitzero"`
}

type cliRouteConn struct {
	ConnOID int    `json:"conn_oid"`
	TID     string `json:"tid"`
}

type cliStopDetails struct {
	Coords      [2]float64 `json:"stop_coords"`
	StopConnOID int        `json:"stop_conn_oid"`
}

type planLegResp struct {
	RouteID             int     `json:"route_id"`
	TripID              string  `json:"trip_id"`
	StartStopID         int     `json:"start_stop_id"`
	DestStopID          int     `json:"dest_stop_id"`
	RideSeconds         float64 `json:"ride_seconds"`
	IntermediateStopIDs []int   `json:"intermediate_stop_ids,omitzero"`
}

type planRouteResp struct {
	Legs               []planLegResp `json:"legs"`
	IsDirect           bool          `json:"is_direct"`
	WalkStartMeters    float64       `json:"walk_start_meters"`
	WalkEndMeters      float64       `json:"walk_end_meters"`
	WalkTransferMeters float64       `json:"walk_transfer_meters"`
	TransitDurationSec float64       `json:"transit_duration_sec"`
	TotalDistance      float64       `json:"total_distance"`
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
	mobrouteMu.RLock()
	ready := mobrouteReady
	mobrouteMu.RUnlock()

	if !ready {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "planner not ready"})
	}

	fromLat, fromLng, toLat, toLng, err := parsePlanCoords(c)
	if err != nil {
		return err
	}

	// Round to ~5 meters precision to increase cache hits for nearby requests.
	const step = 0.000045
	rFromLat := math.Round(fromLat/step) * step
	rFromLng := math.Round(fromLng/step) * step
	rToLat := math.Round(toLat/step) * step
	rToLng := math.Round(toLng/step) * step

	cacheID := fmt.Sprintf("PLAN_%.5f_%.5f_%.5f_%.5f", rFromLat, rFromLng, rToLat, rToLng)
	shelfLife := 5 * time.Minute

	resp, err := HandleCached(cacheID, shelfLife,
		func() (planResp, error) { return getPlanFromDB(cacheID) },
		func() (planResp, error) {
			var allPlans []cliRouteResponse
			startTime := time.Now()
			searchWindow := 30 * time.Minute
			maxIterations := 5

			for range maxIterations {
				if time.Since(startTime) > searchWindow {
					break
				}

				reqObj := map[string]any{
					"feed_ids":            []int{clujFeedID},
					"from":                [2]float64{fromLat, fromLng},
					"to":                  [2]float64{toLat, toLng},
					"time":                startTime.Format(time.RFC3339),
					"transfer_categories": []string{"f", "i", "g"},
					"output_formats":      []string{"legs", "connections"},
				}
				reqJSON, err := json.Marshal(reqObj)
				if err != nil {
					log.Printf("plan_routes: failed to marshal request params: %v", err)
					break
				}

				out, err := runMobroute("route", string(reqJSON))
				if err != nil {
					log.Printf("plan_routes: routing failed: %v", err)
					break
				}

				var cliResp cliRouteResponse
				if err := json.Unmarshal(out, &cliResp); err != nil {
					log.Printf("plan_routes: failed to parse routing response: %v", err)
					break
				}

				if len(cliResp.Legs) == 0 {
					break
				}

				allPlans = append(allPlans, cliResp)

				foundTransit := false
				for _, leg := range cliResp.Legs {
					if leg.LegType == "trip" {
						startTime = leg.LegBeginTime.Add(5 * time.Minute)
						foundTransit = true
						break
					}
				}
				if !foundTransit {
					break
				}
			}

			if len(allPlans) == 0 {
				return planResp{
					Plans:  []planRouteResp{},
					Stops:  map[string]models.Stop{},
					Shapes: map[string]shapeSlim{},
				}, nil
			}

			r, err := enrichResponse(allPlans, tranzyClient, ctpCjClient, cacheTimes)
			if err != nil {
				return planResp{}, err
			}
			return *r, nil
		},
		func(data planResp) error { return storePlanInDB(cacheID, data) },
		CacheOpts[planResp]{},
	)

	if err != nil {
		log.Printf("plan_routes: failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "routing failed"})
	}

	c.Set("Cache-Control", "public, max-age=300")
	return c.JSON(resp)
}

func getPlanFromDB(cacheID string) (planResp, error) {
	var dataStr string
	err := database.DB.QueryRow(`SELECT data FROM directions WHERE id = ?`, cacheID).Scan(&dataStr)
	if err != nil {
		return planResp{}, err
	}

	var data planResp
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return planResp{}, fmt.Errorf("failed to unmarshal plan from DB: %w", err)
	}
	return data, nil
}

func storePlanInDB(cacheID string, data planResp) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal plan for DB: %w", err)
	}

	_, err = database.DB.Exec(`
		INSERT OR REPLACE INTO directions (id, data)
		VALUES (?, ?)
	`, cacheID, string(dataBytes))
	return err
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

func isValidLat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -90 && v <= 90
}

func isValidLng(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -180 && v <= 180
}

func enrichResponse(allPlans []cliRouteResponse, tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) (*planResp, error) {
	if len(allPlans) == 0 {
		return &planResp{Plans: []planRouteResp{}, Stops: map[string]models.Stop{}, Shapes: map[string]shapeSlim{}}, nil
	}

	allStops, err := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{})
	if err != nil {
		return nil, err
	}
	allRoutes, err := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{})
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

	for _, cliResp := range allPlans {
		legs := cliResp.Legs
		var curLegs []planLegResp
		var walkStart, walkEnd, walkTransfer float64
		var transitSec float64

		for i, leg := range legs {
			switch leg.LegType {
			case "walk":
				km := 0.0
				if leg.WalkDistKm != nil {
					km = *leg.WalkDistKm
				}
				meters := km * 1000

				// Check if there is any transit trip later in the plan
				hasTripAfter := false
				for j := i + 1; j < len(legs); j++ {
					if legs[j].LegType == "trip" {
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

			case "transfer":
				if leg.LegDuration != "" {
					if d, pErr := time.ParseDuration(leg.LegDuration); pErr == nil {
						// We don't have distance for pure transfers, but we can estimate
						// or just count the time. The field is meters though.
						// Assume 1.2 m/s walking speed for transfers if no distance.
						walkTransfer += d.Seconds() * 1.2
					}
				}

			case "trip":
				routeName := ""
				if leg.TripRoute != nil {
					routeName = *leg.TripRoute
				}
				route, ok := routeByName[strings.ToUpper(strings.TrimSpace(routeName))]
				if !ok {
					continue
				}

				startStop := nearestStop(allStops, leg.LegFromCoords[0], leg.LegFromCoords[1])
				destStop := nearestStop(allStops, leg.LegToCoords[0], leg.LegToCoords[1])

				var intermediates []int
				if leg.TripStops != nil {
					for _, ts := range *leg.TripStops {
						sid := nearestStop(allStops, ts.Coords[0], ts.Coords[1])
						if sid.StopID != startStop.StopID && sid.StopID != destStop.StopID {
							intermediates = append(intermediates, sid.StopID)
						}
					}
				}

				rideSec := 0.0
				if leg.LegDuration != "" {
					if d, pErr := time.ParseDuration(leg.LegDuration); pErr == nil {
						rideSec = d.Seconds()
						transitSec += rideSec
					}
				}

				tripID := ""
				if leg.TripID != nil {
					tripID = *leg.TripID
				}
				if tripID == "" {
					// Use GetStopInfo to find the trip passing through this stop for the given route.
					if stopInfo, _ := GetStopInfo(tranzyClient, ctpCjClient, cacheTimes, StopFilter{StopID: &startStop.StopID}); stopInfo != nil {
						target0 := strconv.Itoa(route.RouteID) + "_0"
						target1 := strconv.Itoa(route.RouteID) + "_1"

						found0 := slices.Contains(stopInfo.OutgoingTripIds, target0)
						found1 := slices.Contains(stopInfo.IncomingTripIds, target1)

						if found0 && !found1 {
							tripID = target0
						} else if found1 && !found0 {
							tripID = target1
						} else if found0 && found1 {
							// If both match at start, try checking destination stop.
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
				}
				if tripID == "" {
					tripID = strconv.Itoa(route.RouteID) + "_0"
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
			// Deduplicate based on legs sequence
			planKey := fmt.Sprintf("%v", curLegs)
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
