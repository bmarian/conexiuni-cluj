package admin

import (
	"cmp"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/live"
	"conexiuni-cluj/web"
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
	Name  string `json:"name"`
	Used  int    `json:"used"`
	Limit int    `json:"limit"`
}

type adminQuota struct {
	Quotas   []quotaUse `json:"quotas"`
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

func buildOverview(city *cities.City, now time.Time) (*adminOverview, error) {
	st := city.Live.Status(now)
	o := &adminOverview{GeneratedAt: now}
	var err error
	if o.Now, err = nowSection(city, st); err != nil {
		return nil, fmt.Errorf("now: %w", err)
	}
	if o.Accuracy, err = accuracySection(city, now); err != nil {
		return nil, fmt.Errorf("accuracy: %w", err)
	}
	if o.People, err = peopleSection(city, now); err != nil {
		return nil, fmt.Errorf("people: %w", err)
	}
	if o.Quota, err = quotaSection(city, st); err != nil {
		return nil, fmt.Errorf("quota: %w", err)
	}
	if o.Data, err = dataSection(city, st, now); err != nil {
		return nil, fmt.Errorf("data: %w", err)
	}
	if o.Server, err = serverSection(city, now); err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}
	o.Problems = problems(city, st, o, now)
	return o, nil
}

func nowSection(city *cities.City, st live.Status) (adminNow, error) {
	n := adminNow{}
	w := st.Watching
	n.Streams, n.People = w.Streams, w.People
	n.BusesTracked, n.LinesLive = st.BusesTracked, st.LinesLive
	f := st.Feed
	n.BusesInService = f.Valid
	n.FixAgeSec = int(f.FixAge.Seconds())
	if !f.FetchedAt.IsZero() {
		n.FeedAt = &f.FetchedAt
	}
	if f.ErrAt.After(f.FetchedAt) {
		n.FeedError, n.FeedErrorAt = f.Err, &f.ErrAt
	}
	n.PollSec, n.PolledBy = st.PollInterval.Seconds(), st.PolledBy
	var err error
	if n.Stops, err = topNamed(w.Stops, func(ids []int) (map[int]namedCount, error) { return stopNames(city, ids) }); err != nil {
		return n, err
	}
	n.Lines, err = topNamed(w.Routes, func(ids []int) (map[int]namedCount, error) { return routeNames(city, ids) })
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
		mid := bin*live.AccuracyBinSec + live.AccuracyBinSec/2
		signed = append(signed, mass{mid, n})
		abs = append(abs, mass{max(mid, -mid), n})
		s.N += n
		if bin >= -60/live.AccuracyBinSec && bin < 60/live.AccuracyBinSec {
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

func accuracySection(city *cities.City, now time.Time) (adminAccuracy, error) {
	a := adminAccuracy{}
	rows, err := city.MetricRows("eta_error", database.StatsDaysAgo(adminTrendDays-1))
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
	for _, h := range live.AccuracyHorizons {
		a.Today = append(a.Today, get(horizonsToday, h.Label).summarize(h.Label))
		a.Week = append(a.Week, get(horizonsWeek, h.Label).summarize(h.Label))
	}
	a.Today = append(a.Today, allToday.summarize("all"))
	a.Week = append(a.Week, allWeek.summarize("all"))
	for i := adminTrendDays - 1; i >= 0; i-- {
		date := database.StatsDaysAgo(i)
		a.Daily = append(a.Daily, accuracyDay{Date: date, accuracyStats: get(daily, date).summarize(date)})
	}

	rows, err = city.MetricRows("timetable_error", weekStart)
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

func peopleSection(city *cities.City, now time.Time) (adminPeople, error) {
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

	visitors, err := city.MetricRows("visitors", since)
	if err != nil {
		return p, err
	}
	for _, r := range visitors {
		if d := days[r.Date]; d != nil {
			d.Total += r.Count
		}
	}
	newVisitors, err := city.MetricRows("new_visitors", today)
	if err != nil {
		return p, err
	}
	for _, r := range newVisitors {
		p.New += r.Count
	}

	clients, err := city.MetricRows("client_visitors", since)
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

	if p.Stops, err = topMetricToday(city, "stop_view", today, func(ids []int) (map[int]namedCount, error) { return stopNames(city, ids) }); err != nil {
		return p, err
	}
	if p.Lines, err = topMetricToday(city, "route_view", today, func(ids []int) (map[int]namedCount, error) { return routeNames(city, ids) }); err != nil {
		return p, err
	}

	calls, err := city.MetricRows("person_call", today)
	if err != nil {
		return p, err
	}
	for _, r := range calls {
		switch r.Key {
		case "GET /plan":
			p.Plans += r.Count
		case "GET /places":
			p.Places += r.Count
		}
	}
	if p.Installs, err = city.MetricTotal("pwa_install", "appinstalled"); err != nil {
		return p, err
	}
	if p.PushFollowers, p.Reminders, p.ReminderUsers, err = city.Push.Counts(); err != nil {
		return p, err
	}
	p.Moves, err = hostMovesToday(city, today)
	return p, err
}

func hostMovesToday(city *cities.City, today string) (hostMoves, error) {
	var m hostMoves
	rows, err := city.MetricRows("host_move", today)
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
	host := strings.TrimPrefix(web.SiteOrigin, "https://")
	m.OldPush, err = city.Push.SubscribedElsewhere(host)
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

func topMetricToday(city *cities.City, metric, today string, names func([]int) (map[int]namedCount, error)) ([]namedCount, error) {
	rows, err := city.MetricRows(metric, today)
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

func stopNames(city *cities.City, ids []int) (map[int]namedCount, error) {
	names, err := city.StopNames(ids)
	out := make(map[int]namedCount, len(names))
	for id, name := range names {
		out[id] = namedCount{ID: id, Name: name}
	}
	return out, err
}

func routeNames(city *cities.City, ids []int) (map[int]namedCount, error) {
	labels, err := city.RouteLabels(ids)
	out := make(map[int]namedCount, len(labels))
	for id, label := range labels {
		out[id] = namedCount{ID: id, Name: label.ShortName, Color: label.Color}
	}
	return out, err
}

func quotaSection(city *cities.City, st live.Status) (adminQuota, error) {
	q := adminQuota{Quotas: []quotaUse{}}
	seen := map[feeds.Metered]bool{}
	for _, source := range []any{city.Static, city.Timetables, city.Realtime, city.Learning} {
		if metered, ok := source.(feeds.Metered); ok && !seen[metered] {
			seen[metered] = true
			for _, quota := range metered.Quotas() {
				q.Quotas = append(q.Quotas, quotaUse{Name: quota.Name, Used: quota.Used, Limit: quota.Limit})
			}
		}
	}
	learning := st.Learning
	q.Learning.Enabled, q.Learning.Dedicated = learning.Enabled, learning.Dedicated
	q.Learning.Used, q.Learning.Budget = learning.Used, learning.Budget

	return q, nil
}

func dataSection(city *cities.City, st live.Status, now time.Time) (adminData, error) {
	d := adminData{}
	w := city.Ingest.Status()
	if !w.LastCompletedAt.IsZero() {
		d.WarmupAt = &w.LastCompletedAt
	}
	d.WarmupNext = w.NextScheduledAt
	d.WarmupRunning = w.LastStartedAt.After(w.LastCompletedAt)

	briefs, err := city.Updates.Briefs(now.AddDate(0, 0, -7), 20)
	if err != nil {
		return d, err
	}
	d.RouteChanges = make([]routeChangeBrief, len(briefs))
	for i, b := range briefs {
		d.RouteChanges[i] = routeChangeBrief{Line: b.Line, Color: b.Color, DetectedAt: b.DetectedAt, Kinds: b.Kinds}
	}

	var delaysAt time.Time
	d.SamplesToday, d.SamplesHour, delaysAt, err = city.Live.LearningActivity(now)
	if !delaysAt.IsZero() {
		d.DelaysAt = &delaysAt
	}
	d.Archive = archiveSection(city, st.Archive)
	return d, err
}

func archiveSection(city *cities.City, run *live.ArchiveRun) adminArchive {
	a := adminArchive{Enabled: city.Archive != nil}
	if run == nil {
		return a
	}
	a.RanAt, a.Error, a.Bytes = &run.At, run.Err, run.Summary.Bytes
	if run.Summary.OldestAt > 0 {
		a.OldestDate = database.StatsDate(time.Unix(run.Summary.OldestAt, 0))
	}
	return a
}

func serverSection(city *cities.City, now time.Time) (adminServer, error) {
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
	var err error
	if s.DBBytes, err = city.Size(); err != nil {
		return s, err
	}
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
		rows, err := city.MetricRows(metric, today)
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

func problems(city *cities.City, st live.Status, o *adminOverview, now time.Time) []string {
	out := []string{}
	n := o.Now
	if n.FeedError != "" {
		out = append(out, "Live feed failing: "+truncateText(n.FeedError, 160))
	}
	if n.PollSec > 0 {
		last := city.StartedAt()
		if n.FeedAt != nil {
			last = *n.FeedAt
		}
		if gap := now.Sub(last); gap > time.Duration(2*n.PollSec*float64(time.Second)) {
			out = append(out, "No bus positions for "+humanDuration(gap))
		}
	}
	if age := time.Duration(n.FixAgeSec) * time.Second; age > live.MaxVehicleAge {
		out = append(out, "Bus positions are "+humanDuration(age)+" old")
	}
	if n.BusesInService > 0 && n.BusesTracked == 0 {
		out = append(out, fmt.Sprintf("%d buses reported, none matched to a trip", n.BusesInService))
	}
	for _, q := range o.Quota.Quotas {
		if q.Limit > 0 && q.Used >= q.Limit {
			out = append(out, "The "+q.Name+" quota is used up")
		}
	}
	if o.Server.Errors > 0 {
		out = append(out, fmt.Sprintf("%d server errors today", o.Server.Errors))
	}
	if p := st.CalendarProblem; p != "" {
		out = append(out, p)
	}
	if a := o.Data.Archive; !a.Enabled {
		out = append(out, "Archive off, no history kept")
	} else if a.Error != "" {
		out = append(out, "Archive failed: "+truncateText(a.Error, 160))
	} else if a.RanAt == nil && now.Sub(city.StartedAt()) > live.ArchiveStartDelay+time.Hour {
		out = append(out, "Archive not run for "+humanDuration(now.Sub(city.StartedAt())))
	} else if a.RanAt != nil && now.Sub(*a.RanAt) > live.ArchiveOverdue {
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
