package handlers

import (
	"cmp"
	"conexiuni-cluj/database"
	"conexiuni-cluj/services/tranzy"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"
)

var serverStartedAt = time.Now()

const (
	diskFullWarning = 0.9
	adminTopLimit   = 5
	adminPeopleDays = 30
	adminTrendDays  = 14
)

type namedCount struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
	Count int    `json:"count"`
}

type adminOverview struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Problems    []string      `json:"problems"`
	Now         adminNow      `json:"now"`
	Accuracy    adminAccuracy `json:"accuracy"`
	People      adminPeople   `json:"people"`
	Quota       adminQuota    `json:"quota"`
	Data        adminData     `json:"data"`
	Server      adminServer   `json:"server"`
}

type adminNow struct {
	Streams        int          `json:"streams"`
	People         int          `json:"people"`
	BusesTracked   int          `json:"buses_tracked"`
	BusesInService int          `json:"buses_in_service"`
	LinesLive      int          `json:"lines_live"`
	FeedAt         *time.Time   `json:"feed_at"`
	FixAgeSec      int          `json:"fix_age_sec"`
	PollSec        float64      `json:"poll_sec"`
	PolledBy       string       `json:"polled_by"`
	FeedError      string       `json:"feed_error,omitempty"`
	FeedErrorAt    *time.Time   `json:"feed_error_at,omitempty"`
	Stops          []namedCount `json:"stops"`
	Lines          []namedCount `json:"lines"`
}

type accuracyStats struct {
	Label     string  `json:"label"`
	N         int     `json:"n"`
	Within1   float64 `json:"within_1"`
	MedianAbs int     `json:"median_abs_sec"`
	P90Abs    int     `json:"p90_abs_sec"`
	Bias      int     `json:"bias_sec"`
}

type accuracyDay struct {
	Date string `json:"date"`
	accuracyStats
}

type adminAccuracy struct {
	Today     []accuracyStats `json:"today"`
	Week      []accuracyStats `json:"week"`
	Daily     []accuracyDay   `json:"daily"`
	Timetable []accuracyStats `json:"timetable"`
}

type peopleDay struct {
	Date    string `json:"date"`
	Total   int    `json:"total"`
	Web     int    `json:"web"`
	Android int    `json:"android"`
	Wear    int    `json:"wear"`
}

type platformCount struct {
	Platform string `json:"platform"`
	Today    int    `json:"today"`
	LastWeek int    `json:"last_week"`
}

type versionCount struct {
	Platform string `json:"platform"`
	Version  string `json:"version"`
	Today    int    `json:"today"`
	LastSeen string `json:"last_seen"`
}

type adminPeople struct {
	Today         int             `json:"today"`
	New           int             `json:"new"`
	LastWeek      int             `json:"last_week"`
	Platforms     []platformCount `json:"platforms"`
	Daily         []peopleDay     `json:"daily"`
	Versions      []versionCount  `json:"versions"`
	Stops         []namedCount    `json:"stops"`
	Lines         []namedCount    `json:"lines"`
	Plans         int             `json:"plans"`
	Places        int             `json:"places"`
	Installs      int             `json:"installs"`
	PushFollowers int             `json:"push_followers"`
	Reminders     int             `json:"reminders"`
	ReminderUsers int             `json:"reminder_users"`
	Moves         hostMoves       `json:"moves"`
}

// Visits the old host passed on to the canonical one today, and push subscriptions still held there.
type hostMoves struct {
	Sent     int `json:"sent"`
	Empty    int `json:"empty"`
	Received int `json:"received"`
	OldPush  int `json:"old_push"`
}

type quotaUse struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

type adminQuota struct {
	Vehicles quotaUse `json:"vehicles"`
	Default  quotaUse `json:"default"`
	Learning struct {
		Enabled   bool `json:"enabled"`
		Dedicated bool `json:"dedicated"`
		Used      int  `json:"used"`
		Budget    int  `json:"budget"`
	} `json:"learning"`
}

type routeChangeBrief struct {
	Line       string   `json:"line"`
	Color      string   `json:"color"`
	DetectedAt int64    `json:"detected_at"`
	Kinds      []string `json:"kinds"`
}

type adminData struct {
	WarmupAt      *time.Time         `json:"warmup_at"`
	WarmupNext    time.Time          `json:"warmup_next"`
	WarmupRunning bool               `json:"warmup_running"`
	RouteChanges  []routeChangeBrief `json:"route_changes"`
	SamplesToday  int                `json:"samples_today"`
	SamplesHour   int                `json:"samples_hour"`
	DelaysAt      *time.Time         `json:"delays_at"`
	Archive       adminArchive       `json:"archive"`
}

type adminArchive struct {
	Enabled    bool       `json:"enabled"`
	RanAt      *time.Time `json:"ran_at"`
	Error      string     `json:"error,omitempty"`
	Bytes      int64      `json:"bytes"`
	OldestDate string     `json:"oldest_date,omitempty"`
}

type endpointToday struct {
	Key    string  `json:"key"`
	Calls  int     `json:"calls"`
	AvgMs  float64 `json:"avg_ms"`
	Errors int     `json:"errors"`
}

type problemEntry struct {
	First time.Time `json:"first"`
	Last  time.Time `json:"last"`
	Count int       `json:"count"`
	Text  string    `json:"text"`
}

type adminServer struct {
	StartedAt time.Time       `json:"started_at"`
	Revision  string          `json:"revision"`
	Modified  bool            `json:"modified"`
	HeapBytes uint64          `json:"heap_bytes"`
	DBBytes   int64           `json:"db_bytes"`
	DiskFree  uint64          `json:"disk_free"`
	DiskTotal uint64          `json:"disk_total"`
	Requests  int             `json:"requests"`
	Errors    int             `json:"errors"`
	Endpoints []endpointToday `json:"endpoints"`
	Problems  []problemEntry  `json:"problems"`
}

func buildAdminOverview(tranzyClient *tranzy.Client, now time.Time) (*adminOverview, error) {
	o := &adminOverview{GeneratedAt: now}
	var err error
	if o.Now, err = adminNowSection(); err != nil {
		return nil, fmt.Errorf("now: %w", err)
	}
	if o.Accuracy, err = adminAccuracySection(now); err != nil {
		return nil, fmt.Errorf("accuracy: %w", err)
	}
	if o.People, err = adminPeopleSection(now); err != nil {
		return nil, fmt.Errorf("people: %w", err)
	}
	if o.Quota, err = adminQuotaSection(tranzyClient); err != nil {
		return nil, fmt.Errorf("quota: %w", err)
	}
	if o.Data, err = adminDataSection(now); err != nil {
		return nil, fmt.Errorf("data: %w", err)
	}
	if o.Server, err = adminServerSection(now); err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}
	o.Problems = adminProblems(o, now)
	return o, nil
}

func adminNowSection() (adminNow, error) {
	n := adminNow{}
	w := VehicleHub.Watching()
	n.Streams, n.People = w.Streams, w.People
	if table := currentLiveTable(); table != nil {
		lines := make(map[int]struct{})
		for _, vehicles := range table.vehiclesByTrip {
			for _, v := range vehicles {
				n.BusesTracked++
				lines[v.RouteID] = struct{}{}
			}
		}
		n.LinesLive = len(lines)
	}
	f := currentFeedStatus()
	n.BusesInService = f.Valid
	n.FixAgeSec = int(f.FixAge.Seconds())
	if !f.FetchedAt.IsZero() {
		n.FeedAt = &f.FetchedAt
	}
	if f.ErrAt.After(f.FetchedAt) {
		n.FeedError, n.FeedErrorAt = f.Err, &f.ErrAt
	}
	if learning := currentVehicleLearningStatus(); w.Streams > 0 {
		n.PollSec, n.PolledBy = VehicleHub.CurrentInterval().Seconds(), "viewers"
	} else if learning.Enabled {
		n.PollSec, n.PolledBy = learning.Interval.Seconds(), "learner"
	}
	var err error
	if n.Stops, err = topNamed(w.Stops, stopNames); err != nil {
		return n, err
	}
	n.Lines, err = topNamed(w.Routes, routeNames)
	return n, err
}

type errorHistogram map[int]int

func (h errorHistogram) add(bin, count int) { h[bin] += count }

// summarize reads the half-minute bins: bin b holds misses in [30b, 30b+30) seconds.
func (h errorHistogram) summarize(label string) accuracyStats {
	s := accuracyStats{Label: label}
	type mass struct{ sec, n int }
	signed := make([]mass, 0, len(h))
	abs := make([]mass, 0, len(h))
	within := 0
	for bin, n := range h {
		mid := bin*accuracyBinSec + accuracyBinSec/2
		signed = append(signed, mass{mid, n})
		abs = append(abs, mass{max(mid, -mid), n})
		s.N += n
		if bin >= -60/accuracyBinSec && bin < 60/accuracyBinSec {
			within += n
		}
	}
	if s.N == 0 {
		return s
	}
	quantile := func(ms []mass, q float64) int {
		slices.SortFunc(ms, func(a, b mass) int { return cmp.Compare(a.sec, b.sec) })
		target, seen := q*float64(s.N), 0
		for _, m := range ms {
			seen += m.n
			if float64(seen) >= target {
				return m.sec
			}
		}
		return ms[len(ms)-1].sec
	}
	s.Within1 = float64(within) / float64(s.N)
	s.Bias = quantile(signed, 0.5)
	s.MedianAbs = quantile(abs, 0.5)
	s.P90Abs = quantile(abs, 0.9)
	return s
}

func adminAccuracySection(now time.Time) (adminAccuracy, error) {
	a := adminAccuracy{}
	rows, err := database.MetricRows("eta_error", database.StatsDaysAgo(adminTrendDays-1))
	if err != nil {
		return a, err
	}
	today, weekStart := database.StatsDate(now), database.StatsDaysAgo(6)
	horizonsToday, horizonsWeek := map[string]errorHistogram{}, map[string]errorHistogram{}
	daily := map[string]errorHistogram{}
	allToday, allWeek := errorHistogram{}, errorHistogram{}
	get := func(m map[string]errorHistogram, k string) errorHistogram {
		if m[k] == nil {
			m[k] = errorHistogram{}
		}
		return m[k]
	}
	for _, r := range rows {
		parts := strings.Split(r.Key, "|")
		if len(parts) != 3 {
			continue
		}
		bin, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		get(daily, r.Date).add(bin, r.Count)
		if r.Date >= weekStart {
			get(horizonsWeek, parts[1]).add(bin, r.Count)
			allWeek.add(bin, r.Count)
		}
		if r.Date == today {
			get(horizonsToday, parts[1]).add(bin, r.Count)
			allToday.add(bin, r.Count)
		}
	}
	for _, h := range accuracyHorizons {
		a.Today = append(a.Today, get(horizonsToday, h.Label).summarize(h.Label))
		a.Week = append(a.Week, get(horizonsWeek, h.Label).summarize(h.Label))
	}
	a.Today = append(a.Today, allToday.summarize("all"))
	a.Week = append(a.Week, allWeek.summarize("all"))
	for i := adminTrendDays - 1; i >= 0; i-- {
		date := database.StatsDaysAgo(i)
		a.Daily = append(a.Daily, accuracyDay{Date: date, accuracyStats: get(daily, date).summarize(date)})
	}

	rows, err = database.MetricRows("timetable_error", weekStart)
	if err != nil {
		return a, err
	}
	timetable := map[string]errorHistogram{}
	for _, r := range rows {
		source, rawBin, ok := strings.Cut(r.Key, "|")
		bin, err := strconv.Atoi(rawBin)
		if !ok || err != nil {
			continue
		}
		get(timetable, source).add(bin, r.Count)
	}
	for _, source := range []string{"ours", "ctp"} {
		a.Timetable = append(a.Timetable, get(timetable, source).summarize(source))
	}
	return a, nil
}

func clientPlatform(client string) (platform, version string) {
	platform, version, _ = strings.Cut(client, "/")
	return platform, version
}

func adminPeopleSection(now time.Time) (adminPeople, error) {
	p := adminPeople{Versions: []versionCount{}}
	today, lastWeek, since := database.StatsDate(now), database.StatsDaysAgo(7), database.StatsDaysAgo(adminPeopleDays-1)

	days := make(map[string]*peopleDay, adminPeopleDays)
	for i := adminPeopleDays - 1; i >= 0; i-- {
		date := database.StatsDaysAgo(i)
		p.Daily = append(p.Daily, peopleDay{Date: date})
	}
	for i := range p.Daily {
		days[p.Daily[i].Date] = &p.Daily[i]
	}

	visitors, err := database.MetricRows("visitors", since)
	if err != nil {
		return p, err
	}
	for _, r := range visitors {
		if d := days[r.Date]; d != nil {
			d.Total += r.Count
		}
	}
	newVisitors, err := database.MetricRows("new_visitors", today)
	if err != nil {
		return p, err
	}
	for _, r := range newVisitors {
		p.New += r.Count
	}

	clients, err := database.MetricRows("client_visitors", since)
	if err != nil {
		return p, err
	}
	platforms := map[string]*platformCount{}
	for _, name := range []string{"web", "android", "wear"} {
		platforms[name] = &platformCount{Platform: name}
	}
	versions := map[string]*versionCount{}
	for _, r := range clients {
		platform, version := clientPlatform(r.Key)
		if d := days[r.Date]; d != nil {
			switch platform {
			case "web":
				d.Web += r.Count
			case "android":
				d.Android += r.Count
			case "wear":
				d.Wear += r.Count
			default:
				// Scripts counted before robots were left out.
				d.Total = max(0, d.Total-r.Count)
			}
		}
		if pc := platforms[platform]; pc != nil {
			if r.Date == today {
				pc.Today += r.Count
			} else if r.Date == lastWeek {
				pc.LastWeek += r.Count
			}
		}
		if version == "" || platforms[platform] == nil {
			continue
		}
		v := versions[r.Key]
		if v == nil {
			v = &versionCount{Platform: platform, Version: version}
			versions[r.Key] = v
		}
		if r.Date == today {
			v.Today += r.Count
		}
		v.LastSeen = max(v.LastSeen, r.Date)
	}
	for _, name := range []string{"web", "android", "wear"} {
		p.Platforms = append(p.Platforms, *platforms[name])
	}
	if d := days[today]; d != nil {
		p.Today = d.Total
	}
	if d := days[lastWeek]; d != nil {
		p.LastWeek = d.Total
	}
	for _, v := range versions {
		p.Versions = append(p.Versions, *v)
	}
	slices.SortFunc(p.Versions, func(a, b versionCount) int {
		if c := cmp.Compare(a.Platform, b.Platform); c != 0 {
			return c
		}
		return compareVersions(b.Version, a.Version)
	})

	if p.Stops, err = topMetricToday("stop_view", today, stopNames); err != nil {
		return p, err
	}
	if p.Lines, err = topMetricToday("route_view", today, routeNames); err != nil {
		return p, err
	}

	calls, err := database.MetricRows("person_call", today)
	if err != nil {
		return p, err
	}
	for _, r := range calls {
		switch r.Key {
		case "GET /api/plan":
			p.Plans += r.Count
		case "GET /api/places":
			p.Places += r.Count
		}
	}
	if p.Installs, err = database.MetricTotal("pwa_install", "appinstalled"); err != nil {
		return p, err
	}
	err = database.DB.QueryRow(`
		SELECT (SELECT COUNT(DISTINCT endpoint) FROM push_subscription_routes),
		       (SELECT COUNT(*) FROM push_reminders),
		       (SELECT COUNT(DISTINCT endpoint) FROM push_reminders)`).Scan(&p.PushFollowers, &p.Reminders, &p.ReminderUsers)
	if err != nil {
		return p, err
	}
	p.Moves, err = hostMovesToday(today)
	return p, err
}

func hostMovesToday(today string) (hostMoves, error) {
	var m hostMoves
	rows, err := database.MetricRows("host_move", today)
	if err != nil {
		return m, err
	}
	for _, r := range rows {
		switch r.Key {
		case "sent":
			m.Sent += r.Count
		case "empty":
			m.Empty += r.Count
		case "received":
			m.Received += r.Count
		}
	}
	host := strings.TrimPrefix(siteOrigin, "https://")
	err = database.DB.QueryRow(`SELECT COUNT(*) FROM push_subscriptions WHERE host <> ?`, host).Scan(&m.OldPush)
	return m, err
}

func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(as), len(bs)); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if c := cmp.Compare(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func topMetricToday(metric, today string, names func([]int) (map[int]namedCount, error)) ([]namedCount, error) {
	rows, err := database.MetricRows(metric, today)
	if err != nil {
		return nil, err
	}
	counts := make(map[int]int)
	for _, r := range rows {
		if id, err := strconv.Atoi(r.Key); err == nil {
			counts[id] += r.Count
		}
	}
	return topNamed(counts, names)
}

func topNamed(counts map[int]int, names func([]int) (map[int]namedCount, error)) ([]namedCount, error) {
	ids := make([]int, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b int) int {
		if c := cmp.Compare(counts[b], counts[a]); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	ids = ids[:min(len(ids), adminTopLimit)]
	named, err := names(ids)
	if err != nil {
		return nil, err
	}
	out := make([]namedCount, 0, len(ids))
	for _, id := range ids {
		n := named[id]
		n.ID, n.Count = id, counts[id]
		if n.Name == "" {
			n.Name = strconv.Itoa(id)
		}
		out = append(out, n)
	}
	return out, nil
}

func idPlaceholders(ids []int) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

func stopNames(ids []int) (map[int]namedCount, error) {
	out := make(map[int]namedCount, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := idPlaceholders(ids)
	rows, err := queryRows(`SELECT stop_id, stop_name FROM stops WHERE stop_id IN (`+in+`)`, args,
		func(rows *sql.Rows) (namedCount, error) {
			var n namedCount
			err := rows.Scan(&n.ID, &n.Name)
			return n, err
		})
	for _, n := range rows {
		out[n.ID] = n
	}
	return out, err
}

func routeNames(ids []int) (map[int]namedCount, error) {
	out := make(map[int]namedCount, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := idPlaceholders(ids)
	rows, err := queryRows(`SELECT route_id, route_short_name, COALESCE(route_color, '') FROM routes WHERE route_id IN (`+in+`)`, args,
		func(rows *sql.Rows) (namedCount, error) {
			var n namedCount
			err := rows.Scan(&n.ID, &n.Name, &n.Color)
			return n, err
		})
	for _, n := range rows {
		out[n.ID] = n
	}
	return out, err
}

func adminQuotaSection(tranzyClient *tranzy.Client) (adminQuota, error) {
	q := adminQuota{
		Vehicles: quotaUse{Used: tranzyClient.VehiclesQuotaLimit() - tranzyClient.VehiclesQuotaRemaining(), Limit: tranzyClient.VehiclesQuotaLimit()},
		Default:  quotaUse{Used: tranzyClient.DefaultQuotaLimit() - tranzyClient.DefaultQuotaRemaining(), Limit: tranzyClient.DefaultQuotaLimit()},
	}
	learning := currentVehicleLearningStatus()
	q.Learning.Enabled, q.Learning.Dedicated = learning.Enabled, learning.Dedicated
	q.Learning.Used, q.Learning.Budget = learning.Used, learning.Budget

	return q, nil
}

func adminDataSection(now time.Time) (adminData, error) {
	d := adminData{}
	w := WarmupState.Snapshot()
	if !w.LastCompletedAt.IsZero() {
		d.WarmupAt = &w.LastCompletedAt
	}
	d.WarmupNext = w.NextScheduledAt
	d.WarmupRunning = w.LastStartedAt.After(w.LastCompletedAt)

	changes, err := queryRows(`
		SELECT c.route_short_name, COALESCE(r.route_color, ''), c.detected_at, c.changes
		FROM route_changes c
		LEFT JOIN (SELECT route_short_name, MAX(route_color) AS route_color FROM routes GROUP BY route_short_name) r
		  ON r.route_short_name = c.route_short_name
		WHERE c.detected_at >= ?
		ORDER BY c.detected_at DESC
		LIMIT 20`,
		[]any{now.AddDate(0, 0, -7).UnixMilli()},
		func(rows *sql.Rows) (routeChangeBrief, error) {
			b := routeChangeBrief{Kinds: []string{}}
			var payload string
			if err := rows.Scan(&b.Line, &b.Color, &b.DetectedAt, &payload); err != nil {
				return b, err
			}
			var items []RouteChangeItem
			if err := json.Unmarshal([]byte(payload), &items); err != nil {
				return b, err
			}
			for _, it := range items {
				if !slices.Contains(b.Kinds, it.Kind) {
					b.Kinds = append(b.Kinds, it.Kind)
				}
			}
			return b, nil
		})
	if err != nil {
		return d, err
	}
	d.RouteChanges = changes

	loc := database.StatsLocation()
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).Unix()
	var delaysAt int64
	err = database.DB.QueryRow(`
		SELECT (SELECT COUNT(*) FROM segment_travel_time_samples WHERE observed_at >= ?),
		       (SELECT COUNT(*) FROM segment_travel_time_samples WHERE observed_at >= ?),
		       (SELECT COALESCE(MAX(updated_at), 0) FROM schedule_delay_profiles)`,
		midnight, now.Add(-time.Hour).Unix()).Scan(&d.SamplesToday, &d.SamplesHour, &delaysAt)
	if delaysAt > 0 {
		t := time.Unix(delaysAt, 0)
		d.DelaysAt = &t
	}
	d.Archive = adminArchiveSection(lastArchiveRun())
	return d, err
}

func adminArchiveSection(run *archiveRun) adminArchive {
	a := adminArchive{Enabled: database.Archive != nil}
	if run == nil {
		return a
	}
	a.RanAt, a.Error, a.Bytes = &run.At, run.Err, run.Summary.Bytes
	if run.Summary.OldestAt > 0 {
		a.OldestDate = database.StatsDate(time.Unix(run.Summary.OldestAt, 0))
	}
	return a
}

func adminServerSection(now time.Time) (adminServer, error) {
	s := adminServer{StartedAt: serverStartedAt, Endpoints: []endpointToday{}, Problems: []problemEntry{}}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, kv := range info.Settings {
			switch kv.Key {
			case "vcs.revision":
				s.Revision = kv.Value[:min(len(kv.Value), 8)]
			case "vcs.modified":
				s.Modified = kv.Value == "true"
			}
		}
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	s.HeapBytes = mem.HeapAlloc
	var pages, pageSize int64
	if err := database.DB.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		return s, err
	}
	if err := database.DB.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		return s, err
	}
	s.DBBytes = pages * pageSize
	s.DiskFree, s.DiskTotal, _ = diskSpace(".")

	today := database.StatsDate(now)
	endpoints := map[string]*endpointToday{}
	get := func(key string) *endpointToday {
		if endpoints[key] == nil {
			endpoints[key] = &endpointToday{Key: key}
		}
		return endpoints[key]
	}
	totalMs := map[string]int{}
	for _, metric := range []string{"endpoint_call", "endpoint_ms", "endpoint_error"} {
		rows, err := database.MetricRows(metric, today)
		if err != nil {
			return s, err
		}
		for _, r := range rows {
			switch metric {
			case "endpoint_call":
				get(r.Key).Calls += r.Count
				s.Requests += r.Count
			case "endpoint_ms":
				totalMs[r.Key] += r.Count
			case "endpoint_error":
				get(r.Key).Errors += r.Count
				s.Errors += r.Count
			}
		}
	}
	for key, e := range endpoints {
		if e.Calls > 0 && totalMs[key] > 0 {
			e.AvgMs = math.Round(float64(totalMs[key])/float64(e.Calls)*10) / 10
		}
		s.Endpoints = append(s.Endpoints, *e)
	}
	slices.SortFunc(s.Endpoints, func(a, b endpointToday) int {
		if c := cmp.Compare(b.Calls, a.Calls); c != 0 {
			return c
		}
		return cmp.Compare(a.Key, b.Key)
	})
	for _, l := range ProblemLog.Recent(now.Add(-24 * time.Hour)) {
		s.Problems = append(s.Problems, problemEntry{First: l.First, Last: l.Last, Count: l.Count, Text: l.Text})
	}
	return s, nil
}

func adminProblems(o *adminOverview, now time.Time) []string {
	out := []string{}
	n := o.Now
	if n.FeedError != "" {
		out = append(out, "Tranzy failing: "+truncateText(n.FeedError, 160))
	}
	if n.PollSec > 0 {
		last := serverStartedAt
		if n.FeedAt != nil {
			last = *n.FeedAt
		}
		if gap := now.Sub(last); gap > time.Duration(2*n.PollSec*float64(time.Second)) {
			out = append(out, "No bus positions for "+humanDuration(gap))
		}
	}
	if age := time.Duration(n.FixAgeSec) * time.Second; age > maxObservedVehicleAge {
		out = append(out, "Bus positions are "+humanDuration(age)+" old")
	}
	if n.BusesInService > 0 && n.BusesTracked == 0 {
		out = append(out, fmt.Sprintf("%d buses reported, none matched to a trip", n.BusesInService))
	}
	if q := o.Quota.Vehicles; q.Limit > 0 && q.Used >= q.Limit {
		out = append(out, "Tranzy vehicle quota used up")
	}
	if o.Server.Errors > 0 {
		out = append(out, fmt.Sprintf("%d server errors today", o.Server.Errors))
	}
	if p := calendarProblem(now); p != "" {
		out = append(out, p)
	}
	if a := o.Data.Archive; !a.Enabled {
		out = append(out, "Archive off, no history kept")
	} else if a.Error != "" {
		out = append(out, "Archive failed: "+truncateText(a.Error, 160))
	} else if a.RanAt == nil && now.Sub(serverStartedAt) > archiveStartDelay+time.Hour {
		out = append(out, "Archive not run for "+humanDuration(now.Sub(serverStartedAt)))
	} else if a.RanAt != nil && now.Sub(*a.RanAt) > archiveOverdue {
		out = append(out, "Archive not run for "+humanDuration(now.Sub(*a.RanAt)))
	}
	if s := o.Server; s.DiskTotal > 0 {
		if used := 1 - float64(s.DiskFree)/float64(s.DiskTotal); used >= diskFullWarning {
			out = append(out, fmt.Sprintf("Disk %.0f%% full", used*100))
		}
	}
	for _, p := range o.Server.Problems {
		if now.Sub(p.Last) < time.Hour {
			out = append(out, "Failures logged in the last hour")
			break
		}
	}
	return out
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}
