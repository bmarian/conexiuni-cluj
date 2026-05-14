package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
	"time"

	"conexiuni-cluj/database"
	"conexiuni-cluj/services/tranzy"

	"github.com/gofiber/fiber/v3"
)

const (
	adminSessionCookie = "conexiuni_admin"
	adminSessionMaxAge = 12 * time.Hour
)

type adminLoginRequest struct {
	Token string `json:"token"`
}

func adminTokenMatches(token, candidate string) bool {
	expected := []byte(token)
	got := []byte(strings.TrimSpace(candidate))
	return subtle.ConstantTimeEq(int32(len(got)), int32(len(expected))) == 1 &&
		subtle.ConstantTimeCompare(got, expected) == 1
}

func adminSessionSignature(token, expires, nonce string) string {
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(expires))
	_, _ = mac.Write([]byte("|"))
	_, _ = mac.Write([]byte(nonce))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func newAdminSession(token string, now time.Time) string {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		nonceBytes = []byte(strconv.FormatInt(now.UnixNano(), 36))
	}
	expires := strconv.FormatInt(now.Add(adminSessionMaxAge).Unix(), 10)
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	return expires + "." + nonce + "." + adminSessionSignature(token, expires, nonce)
}

func validAdminSession(token, value string, now time.Time) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || !now.Before(time.Unix(expUnix, 0)) {
		return false
	}
	expected := []byte(adminSessionSignature(token, parts[0], parts[1]))
	got := []byte(parts[2])
	return subtle.ConstantTimeEq(int32(len(got)), int32(len(expected))) == 1 &&
		subtle.ConstantTimeCompare(got, expected) == 1
}

func setAdminSessionCookie(c fiber.Ctx, token string, secure bool) {
	now := time.Now()
	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    newAdminSession(token, now),
		Path:     "/api/admin",
		Expires:  now.Add(adminSessionMaxAge),
		MaxAge:   int(adminSessionMaxAge.Seconds()),
		Secure:   secure,
		HTTPOnly: true,
		SameSite: "Strict",
	})
}

func clearAdminSessionCookie(c fiber.Ctx, secure bool) {
	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    "",
		Path:     "/api/admin",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   secure,
		HTTPOnly: true,
		SameSite: "Strict",
	})
}

func RequireAdminSession(token string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if token == "" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "admin disabled"})
		}
		if validAdminSession(token, c.Cookies(adminSessionCookie), time.Now()) {
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
}

type adminStatsResponse struct {
	Visitors              database.VisitorTotals           `json:"visitors"`
	DailyVisits           []database.DailyVisitorPoint     `json:"daily_visits"`
	ActiveNow             int                              `json:"active_now"`
	TopRoutes             []database.TopEntry              `json:"top_routes"`
	TopStops              []database.TopEntry              `json:"top_stops"`
	TopAPI                []database.TopEntry              `json:"top_api"`
	PWAInstalls           int                              `json:"pwa_installs"`
	TranzyQuota           tranzyQuotaSnapshot              `json:"tranzy_quota"`
	DailyTranzyQuota      []database.DailyTranzyQuotaPoint `json:"daily_tranzy_quota"`
	EndpointResponseTimes []database.EndpointTimingEntry   `json:"endpoint_response_times"`
	CacheGroups           []cacheGroupSnapshot             `json:"cache_groups"`
	SegmentLearning       segmentLearningStats             `json:"segment_learning"`
	Warmup                warmupSnapshotResponse           `json:"warmup"`
	GeneratedAt           string                           `json:"generated_at"`
}

type cacheGroupSnapshot struct {
	Prefix            string `json:"prefix"`
	Count             int    `json:"count"`
	ExpiredCount      int    `json:"expired_count"`
	EarliestExpiresAt string `json:"earliest_expires_at"`
	LatestExpiresAt   string `json:"latest_expires_at"`
	LifespanMs        int64  `json:"lifespan_ms"`
}

type warmupSnapshotResponse struct {
	LastStartedAt   string `json:"last_started_at,omitempty"`
	LastCompletedAt string `json:"last_completed_at,omitempty"`
	LastDurationMs  int64  `json:"last_duration_ms,omitempty"`
	NextScheduledAt string `json:"next_scheduled_at"`
}

type segmentLearningStats struct {
	TotalSamples       int                            `json:"total_samples"`
	TotalProfiles      int                            `json:"total_profiles"`
	RoutesWithProfiles int                            `json:"routes_with_profiles"`
	LastSampleAt       string                         `json:"last_sample_at,omitempty"`
	LastSnapshot       segmentLearningRuntimeSnapshot `json:"last_snapshot"`
	TopRoutes          []segmentLearningRouteStats    `json:"top_routes"`
}

type segmentLearningRouteStats struct {
	RouteID        int    `json:"route_id"`
	RouteShortName string `json:"route_short_name"`
	Samples        int    `json:"samples"`
	Profiles       int    `json:"profiles"`
}

type tranzyQuotaSnapshot struct {
	VehiclesRemaining int `json:"vehicles_remaining"`
	VehiclesLimit     int `json:"vehicles_limit"`
	VehiclesUsed      int `json:"vehicles_used"`
	DefaultRemaining  int `json:"default_remaining"`
	DefaultLimit      int `json:"default_limit"`
	DefaultUsed       int `json:"default_used"`
}

func quotaUsed(limit, remaining int) int {
	used := limit - remaining
	if used < 0 {
		return 0
	}
	return used
}

func mergeTodayTranzyQuotaUsage(points []database.DailyTranzyQuotaPoint, loc *time.Location, vehiclesUsed, defaultUsed int) []database.DailyTranzyQuotaPoint {
	today := time.Now().In(loc).Format("2006-01-02")
	for i := range points {
		if points[i].Date != today {
			continue
		}
		if points[i].Vehicles < vehiclesUsed {
			points[i].Vehicles = vehiclesUsed
		}
		if points[i].Default < defaultUsed {
			points[i].Default = defaultUsed
		}
		points[i].Total = points[i].Vehicles + points[i].Default
		return points
	}
	return append(points, database.DailyTranzyQuotaPoint{
		Date:     today,
		Vehicles: vehiclesUsed,
		Default:  defaultUsed,
		Total:    vehiclesUsed + defaultUsed,
	})
}

// Longest prefixes first so "API_STOP_TIMES" matches before "STOP_TIMES".
var knownCachePrefixes = []string{
	"API_STOP_TIMES",
	"STOP_TIMES",
	"STOP_INFO",
	"TIMETABLE",
	"ROUTES",
	"STOPS",
	"SHAPES",
	"TRIPS",
	"VEHICLES",
	"news",
}

func cachePrefix(id string) string {
	for _, p := range knownCachePrefixes {
		if id == p || strings.HasPrefix(id, p+"_") {
			return p
		}
	}
	return id
}

func buildCacheGroups(entries []database.CacheEntry, now time.Time) []cacheGroupSnapshot {
	type agg struct {
		count       int
		expired     int
		earliestExp int64
		latestExp   int64
		lifespan    int64
		hasEarliest bool
		hasLatest   bool
	}
	groups := make(map[string]*agg)
	order := make([]string, 0)
	nowMs := now.UnixMilli()
	for _, e := range entries {
		prefix := cachePrefix(e.ID)
		g, ok := groups[prefix]
		if !ok {
			g = &agg{}
			groups[prefix] = g
			order = append(order, prefix)
		}
		g.count++
		exp := e.Timestamp + e.Lifespan
		if exp < nowMs {
			g.expired++
		}
		if !g.hasEarliest || exp < g.earliestExp {
			g.earliestExp = exp
			g.hasEarliest = true
		}
		if !g.hasLatest || exp > g.latestExp {
			g.latestExp = exp
			g.hasLatest = true
		}
		if g.lifespan == 0 {
			g.lifespan = e.Lifespan
		}
	}
	out := make([]cacheGroupSnapshot, 0, len(order))
	for _, prefix := range order {
		g := groups[prefix]
		out = append(out, cacheGroupSnapshot{
			Prefix:            prefix,
			Count:             g.count,
			ExpiredCount:      g.expired,
			EarliestExpiresAt: time.UnixMilli(g.earliestExp).UTC().Format(time.RFC3339),
			LatestExpiresAt:   time.UnixMilli(g.latestExp).UTC().Format(time.RFC3339),
			LifespanMs:        g.lifespan,
		})
	}
	return out
}

func buildWarmupResponse(s WarmupSnapshot) warmupSnapshotResponse {
	resp := warmupSnapshotResponse{
		NextScheduledAt: s.NextScheduledAt.UTC().Format(time.RFC3339),
	}
	if !s.LastStartedAt.IsZero() {
		resp.LastStartedAt = s.LastStartedAt.UTC().Format(time.RFC3339)
	}
	if !s.LastCompletedAt.IsZero() {
		resp.LastCompletedAt = s.LastCompletedAt.UTC().Format(time.RFC3339)
		resp.LastDurationMs = s.LastDuration.Milliseconds()
	}
	return resp
}

func loadSegmentLearningStats(loc *time.Location) (segmentLearningStats, error) {
	if loc == nil {
		loc = time.Local
	}
	stats := segmentLearningStats{
		LastSnapshot: currentSegmentLearningSnapshot(),
		TopRoutes:    []segmentLearningRouteStats{},
	}

	var lastObserved int64
	if err := database.DB.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT route_id), COALESCE(MAX(observed_at), 0)
		FROM segment_travel_time_samples
	`).Scan(&stats.TotalSamples, &stats.RoutesWithProfiles, &lastObserved); err != nil {
		return stats, err
	}
	if lastObserved > 0 {
		stats.LastSampleAt = time.Unix(lastObserved, 0).In(loc).Format(time.RFC3339)
	}

	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM segment_travel_time_profiles`).Scan(&stats.TotalProfiles); err != nil {
		return stats, err
	}

	rows, err := database.DB.Query(`
		SELECT
			p.route_id,
			COALESCE(r.route_short_name, ''),
			COUNT(*) AS profiles,
			COALESCE(MAX(s.sample_count), 0) AS samples
		FROM segment_travel_time_profiles p
		LEFT JOIN routes r ON r.route_id = p.route_id
		LEFT JOIN (
			SELECT route_id, COUNT(*) AS sample_count
			FROM segment_travel_time_samples
			GROUP BY route_id
		) s ON s.route_id = p.route_id
		GROUP BY p.route_id, r.route_short_name
		ORDER BY profiles DESC, samples DESC
		LIMIT 8
	`)
	if err != nil {
		return stats, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var entry segmentLearningRouteStats
		if err := rows.Scan(&entry.RouteID, &entry.RouteShortName, &entry.Profiles, &entry.Samples); err != nil {
			return stats, err
		}
		if entry.RouteShortName == "" {
			entry.RouteShortName = strconv.Itoa(entry.RouteID)
		}
		stats.TopRoutes = append(stats.TopRoutes, entry)
	}
	return stats, rows.Err()
}

// mergeTopEntries collapses entries whose keys map to the same final value
// (after `resolve`) and drops anything `keep` rejects. Old stats_daily rows
// recorded before the Availability filter landed still contain retired routes
// and stops — this filters them at display time so bots probing dead URLs
// don't pollute the top lists.
func mergeTopEntries(entries []database.TopEntry, resolve func(string) string, keep func(string) bool) []database.TopEntry {
	indexByKey := make(map[string]int, len(entries))
	out := make([]database.TopEntry, 0, len(entries))
	for _, e := range entries {
		key := resolve(e.Key)
		if !keep(key) {
			continue
		}
		if idx, ok := indexByKey[key]; ok {
			out[idx].Count += e.Count
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, database.TopEntry{Key: key, Count: e.Count})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func normalizeTopRouteKeys(entries []database.TopEntry) []database.TopEntry {
	resolve := func(key string) string {
		id, err := strconv.Atoi(key)
		if err != nil {
			return key
		}
		var shortName string
		err = database.DB.QueryRow(`SELECT route_short_name FROM routes WHERE route_id = ?`, id).Scan(&shortName)
		if err == nil && strings.TrimSpace(shortName) != "" {
			return shortName
		}
		return key
	}
	keep := func(key string) bool {
		if !Availability.IsReady() {
			return true
		}
		return Availability.RouteHasTimetable(key)
	}
	return mergeTopEntries(entries, resolve, keep)
}

func normalizeTopStopKeys(entries []database.TopEntry) []database.TopEntry {
	resolve := func(key string) string {
		id, err := strconv.Atoi(key)
		if err != nil {
			return key
		}
		var name string
		err = database.DB.QueryRow(`SELECT stop_name FROM stops WHERE stop_id = ?`, id).Scan(&name)
		if err == nil && strings.TrimSpace(name) != "" {
			return name
		}
		return key
	}
	keep := func(key string) bool {
		if !Availability.IsReady() {
			return true
		}
		id, err := strconv.Atoi(key)
		if err == nil {
			return Availability.StopHasBuses(id)
		}
		var stopID int
		err = database.DB.QueryRow(`SELECT stop_id FROM stops WHERE stop_name = ? LIMIT 1`, key).Scan(&stopID)
		if err != nil {
			return false
		}
		return Availability.StopHasBuses(stopID)
	}
	return mergeTopEntries(entries, resolve, keep)
}

func RegisterAdminRoutes(api fiber.Router, token string, tranzyClient *tranzy.Client, secureCookies bool) {
	if token == "" {
		return
	}
	admin := api.Group("/admin")

	admin.Post("/login", func(c fiber.Ctx) error {
		var req adminLoginRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
		if !adminTokenMatches(token, req.Token) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		setAdminSessionCookie(c, token, secureCookies)
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"ok": true})
	})

	admin.Post("/logout", func(c fiber.Ctx) error {
		clearAdminSessionCookie(c, secureCookies)
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"ok": true})
	})

	admin.Get("/stats", RequireAdminSession(token), func(c fiber.Ctx) error {
		visitors, err := database.GetVisitorTotals()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		daily, err := database.GetDailyVisitors(30)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		topRoutes, err := database.GetTopMetric("route_view", 25)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		topRoutes = normalizeTopRouteKeys(topRoutes)
		topStops, err := database.GetTopMetric("stop_view", 25)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		topStops = normalizeTopStopKeys(topStops)
		topAPI, err := database.GetTopMetric("api_call", 25)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		pwaInstalls, err := database.GetMetricTotal("pwa_install", "appinstalled")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		dailyTranzyQuota, err := database.GetDailyTranzyQuotaUsage(30)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		endpointResponseTimes, err := database.GetEndpointResponseTimes(25)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		vehiclesRemaining := tranzyClient.VehiclesQuotaRemaining()
		vehiclesLimit := tranzyClient.VehiclesQuotaLimit()
		defaultRemaining := tranzyClient.DefaultQuotaRemaining()
		defaultLimit := tranzyClient.DefaultQuotaLimit()
		vehiclesUsed := quotaUsed(vehiclesLimit, vehiclesRemaining)
		defaultUsed := quotaUsed(defaultLimit, defaultRemaining)
		dailyTranzyQuota = mergeTodayTranzyQuotaUsage(dailyTranzyQuota, tranzyClient.Location(), vehiclesUsed, defaultUsed)
		cacheEntries, err := database.ListCacheEntries()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		cacheGroups := buildCacheGroups(cacheEntries, time.Now())
		segmentLearning, err := loadSegmentLearningStats(tranzyClient.Location())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		warmup := buildWarmupResponse(WarmupState.Snapshot())

		resp := adminStatsResponse{
			Visitors:    visitors,
			DailyVisits: daily,
			ActiveNow:   VehicleHub.SubscriberCount(),
			TopRoutes:   topRoutes,
			TopStops:    topStops,
			TopAPI:      topAPI,
			PWAInstalls: pwaInstalls,
			TranzyQuota: tranzyQuotaSnapshot{
				VehiclesRemaining: vehiclesRemaining,
				VehiclesLimit:     vehiclesLimit,
				VehiclesUsed:      vehiclesUsed,
				DefaultRemaining:  defaultRemaining,
				DefaultLimit:      defaultLimit,
				DefaultUsed:       defaultUsed,
			},
			DailyTranzyQuota:      dailyTranzyQuota,
			EndpointResponseTimes: endpointResponseTimes,
			CacheGroups:           cacheGroups,
			SegmentLearning:       segmentLearning,
			Warmup:                warmup,
			GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(resp)
	})

}
