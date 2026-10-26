package main

import (
	"bytes"
	"compress/gzip"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/config"
	"conexiuni-cluj/devicesync"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"conexiuni-cluj/planner"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gofiber/fiber/v3"
)

// PARITY_DATA holds db/ and archive/ copies of every city; each API answer for them is written to PARITY_OUT.
// PARITY_PBF optionally points at romania-latest.osm.pbf; without it the planner walks a synthetic street grid.
func TestAPIParity(t *testing.T) {
	data, out := os.Getenv("PARITY_DATA"), os.Getenv("PARITY_OUT")
	if data == "" || out == "" {
		t.Skip("PARITY_DATA and PARITY_OUT not set")
	}
	data, err := filepath.Abs(data)
	if err != nil {
		t.Fatal(err)
	}
	if out, err = filepath.Abs(out); err != nil {
		t.Fatal(err)
	}
	work := parityWorkdir(t)
	for _, dir := range []string{"db", "archive"} {
		copyDir(t, filepath.Join(data, dir), filepath.Join(work, dir))
	}
	for _, slug := range []string{"cluj", "iasi", "oradea", "constanta"} {
		t.Setenv("TRANZY_API_KEY_"+strings.ToUpper(slug), "parity")
	}
	parityOSM(t, work, storedOSMStops(t, filepath.Join(work, "db")))
	now := latestObservation(t, filepath.Join(work, "db")).Add(30 * time.Second)
	morning := time.Date(now.Year(), now.Month(), now.Day()+1, 7, 40, 0, 0, time.Local)
	cfg := config.Load()
	recordParity(t, out, now, morning, cfg, func() ([]*cities.City, error) {
		built, err := buildCities(cfg)
		if err != nil {
			return nil, err
		}
		for _, city := range built {
			freezeSources(t, city)
		}
		return built, nil
	})
}

const parityIndexHTML = `<!DOCTYPE html>
<html lang="ro">
<head>
<title>index</title>
<meta name="description" content="index">
<link rel="canonical" href="https://ceautobuziau.ro/">
<meta property="og:title" content="index">
<meta property="og:description" content="index">
<meta property="og:url" content="https://ceautobuziau.ro/">
<meta name="twitter:title" content="index">
<meta name="twitter:description" content="index">
</head>
<body><div id="app"></div></body>
</html>
`

var parityFileTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// As main sets TZ, and before anything reads the local zone.
func init() {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		panic(err)
	}
	time.Local = loc
}

// A tree shaped like the deployed one, with the server run from backend/.
func parityWorkdir(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	dist := filepath.Join(work, "backend", "dist")
	files := map[string]string{
		filepath.Join(work, "keys.env"):             "",
		filepath.Join(work, ".env"):                 "ENV=development\nADMIN_TOKEN=parity\nLOG_IP_HASH_SALT=parity\n",
		filepath.Join(dist, "index.html"):           parityIndexHTML,
		filepath.Join(dist, "robots.txt"):           "User-agent: *\nDisallow: /admin\n",
		filepath.Join(dist, "sw.js"):                "// sw\n",
		filepath.Join(dist, "manifest.webmanifest"): "{}\n",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, parityFileTime, parityFileTime); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(filepath.Join(work, "backend"))
	return work
}

func parityOSM(t *testing.T, work string, stops map[string][]osmStop) {
	t.Helper()
	path := filepath.Join(work, "backend", planner.OSMPath())
	pbf := os.Getenv("PARITY_PBF")
	if pbf == "" {
		writeSyntheticOSM(t, path, stops)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(pbf, path); err != nil {
		t.Fatal(err)
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The newest vehicle in any city, so the frozen buses are still fresh.
func latestObservation(t *testing.T, dbDir string) time.Time {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(dbDir, "*.db"))
	var latest int64
	for _, path := range paths {
		db, err := sql.Open("sqlite3", path)
		if err != nil {
			t.Fatal(err)
		}
		var at sql.NullInt64
		if err := db.QueryRow(`SELECT MAX(observed_at) FROM vehicles`).Scan(&at); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
		latest = max(latest, at.Int64)
	}
	if latest == 0 {
		t.Fatal("no vehicles to freeze the clock at")
	}
	return time.UnixMilli(latest).Truncate(time.Second)
}

var errFrozen = errors.New("frozen")

// Answers like the source it stands for without ever calling out; every holder of a source gets the same stand-in.
type frozen struct {
	inner    any
	vehicles []models.Vehicle
}

func (f *frozen) Snapshot() (*feeds.Snapshot, error) { return nil, errFrozen }
func (f *frozen) Timetables(*feeds.Snapshot) (map[string]*models.Timetable, error) {
	return nil, errFrozen
}
func (f *frozen) News(*time.Location) ([]feeds.NewsItem, error) { return nil, errFrozen }
func (f *frozen) Vehicles() ([]models.Vehicle, error) {
	return append([]models.Vehicle(nil), f.vehicles...), nil
}

func (f *frozen) Cadence() time.Duration {
	if c, ok := f.inner.(interface{ Cadence() time.Duration }); ok {
		return c.Cadence()
	}
	return 24 * time.Hour
}

func (f *frozen) Budget() feeds.Budget {
	if r, ok := f.inner.(feeds.Realtime); ok {
		return r.Budget()
	}
	return feeds.Budget{}
}

type frozenCredited struct{ *frozen }

func (f frozenCredited) Credit() feeds.Credit { return f.inner.(feeds.Credited).Credit() }
func (f frozenCredited) Quotas() []feeds.Quota {
	if m, ok := f.inner.(feeds.Metered); ok {
		return m.Quotas()
	}
	return nil
}

type freezer struct {
	vehicles []models.Vehicle
	seen     map[any]any
}

func (z *freezer) wrap(inner any) any {
	if inner == nil {
		return nil
	}
	if w, ok := z.seen[inner]; ok {
		return w
	}
	var w any = &frozen{inner: inner, vehicles: z.vehicles}
	if _, ok := inner.(feeds.Credited); ok {
		w = frozenCredited{w.(*frozen)}
	}
	z.seen[inner] = w
	return w
}

func freezeSources(t *testing.T, city *cities.City) {
	t.Helper()
	z := &freezer{vehicles: storedVehicles(t, city.DB), seen: map[any]any{}}
	if city.Static != nil {
		city.Static = z.wrap(city.Static).(feeds.Static)
	}
	if city.Timetables != nil {
		city.Timetables = z.wrap(city.Timetables).(feeds.Timetables)
	}
	if city.Realtime != nil {
		city.Realtime = z.wrap(city.Realtime).(feeds.Realtime)
	}
	if city.Learning != nil {
		city.Learning = z.wrap(city.Learning).(feeds.Realtime)
	}
	if city.News != nil {
		city.News = z.wrap(city.News).(feeds.News)
	}
	city.Ingest.Static, city.Ingest.Timetables = city.Static, city.Timetables
	city.Live.Realtime, city.Live.Learning = city.Realtime, city.Learning
	city.Live.SchoolCalendar = nil
}

func storedVehicles(t *testing.T, db *sql.DB) []models.Vehicle {
	t.Helper()
	rows, err := db.Query(`SELECT id, label, latitude, longitude, timestamp, vehicle_type, bike_accessible, wheelchair_accessible,
		raw_speed, route_id, trip_id FROM vehicles ORDER BY CAST(id AS INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		if err := rows.Scan(&v.ID, &v.Label, &v.Latitude, &v.Longitude, &v.Timestamp, &v.VehicleType, &v.BikeAccessible,
			&v.WheelchairAccessible, &v.Speed, &v.RouteID, &v.TripID); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// Runs the server on a fake clock set to now, then again at later, and writes every answer under out.
func recordParity(t *testing.T, out string, now, later time.Time, cfg *config.Config, build func() ([]*cities.City, error)) {
	t.Helper()
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile, err := os.Create(out + ".log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	defer log.SetOutput(os.Stderr)

	// These start goroutines that never end; started out here, they stay out of the bubble.
	_, _ = fiber.New().Test(httptest.NewRequest(http.MethodGet, "/", nil))
	_ = accessLogger(io.Discard, "parity")
	outside, served := make(chan *http.Request), make(chan struct{})
	defer close(outside)
	var app *fiber.App
	go func() {
		for req := range outside {
			_, _ = app.Test(req, fiber.TestConfig{Timeout: 0})
			served <- struct{}{}
		}
	}()

	synctest.Test(t, func(t *testing.T) {
		time.Sleep(time.Until(now))
		built, err := build()
		if err != nil {
			t.Fatal(err)
		}
		registry, err := cities.NewRegistry(built, filepath.Join(cfg.DBDir, "disabled-cities.json"))
		if err != nil {
			t.Fatal(err)
		}
		devices, err := devicesync.Open(filepath.Join(cfg.DBDir, "sync.db"), devicesync.Options{Cities: len(built), MaxBytes: int64(cfg.SyncDBMaxMB) << 20})
		if err != nil {
			t.Fatal(err)
		}
		app = newApp(cfg, built, registry, devices, io.Discard, "parity")
		for _, path := range []string{"/robots.txt", "/nope.png"} {
			outside <- httptest.NewRequest(http.MethodGet, path, nil)
			<-served
		}
		registry.Start()
		devices.Start()
		synctest.Wait()

		rec := &recorder{t: t, app: app, out: out, names: map[string]int{}}
		rec.routes()
		rec.walk(built, cfg.AdminToken)
		rec.walkWrites(built, cfg.AdminToken)

		// Through the night, so the nightly archive, vacuum and relearning run too.
		time.Sleep(time.Until(later))
		synctest.Wait()
		rec.prefix = "morning "
		rec.walkMorning(built, cfg.AdminToken)

		registry.Shutdown()
		if err := devices.Close(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		for _, city := range built {
			city.Close()
		}
	})
}

type recorder struct {
	t      *testing.T
	prefix string
	// Random values in answers, replaced wherever they show.
	masks  [][2]string
	app    *fiber.App
	out    string
	names  map[string]int
	cookie string
}

type bundleShape struct {
	Routes []struct {
		RouteID        int    `json:"route_id"`
		RouteShortName string `json:"route_short_name"`
		Slug           string `json:"slug"`
		Trips          []struct {
			TripID string `json:"trip_id"`
		} `json:"trips"`
	} `json:"routes"`
	Stops []struct {
		StopID   int     `json:"stop_id"`
		StopName string  `json:"stop_name"`
		Lat      float64 `json:"stop_lat"`
		Lon      float64 `json:"stop_lon"`
		Slug     string  `json:"slug"`
	} `json:"stops"`
}

func (r *recorder) walk(built []*cities.City, adminToken string) {
	r.get("/api/cities")
	for _, city := range built {
		r.walkCity(city.Slug)
	}
	for _, path := range []string{"/sitemap.xml", "/robots.txt", "/sw.js", "/manifest.webmanifest", "/.well-known/assetlinks.json",
		"/stop/1", "/route/1/0", "/plan?lat=46.77&lon=23.6&name=x", "/settings", "/api/nope", "/nope.png", "/nope"} {
		r.get(path)
	}
	if adminToken == "" {
		return
	}
	flushStats(r.t, built)
	r.do(http.MethodPost, "/api/admin/login", `{"token":"wrong"}`, "Content-Type", "application/json")
	r.get("/api/admin/overview?city=" + built[0].Slug)
	r.do(http.MethodPost, "/api/admin/login", `{"token":"`+adminToken+`"}`, "Content-Type", "application/json")
	r.get("/api/admin/cities")
	r.get("/api/admin/site")
	for _, city := range built {
		r.get("/api/admin/overview?city=" + city.Slug)
		r.get("/api/admin/calendar?city=" + city.Slug)
	}
	r.do(http.MethodPost, "/api/admin/logout", "")
}

func (r *recorder) walkMorning(built []*cities.City, adminToken string) {
	r.get("/api/cities")
	for _, city := range built {
		api := "/api/" + city.Slug
		var bundle bundleShape
		if err := json.Unmarshal(r.get(api+"/network"), &bundle); err != nil {
			r.t.Fatalf("%s network: %v", city.Slug, err)
		}
		r.get(api + "/network/version")
		var trips []string
		for _, route := range bundle.Routes {
			for _, trip := range route.Trips {
				trips = append(trips, trip.TripID)
			}
		}
		sort.Strings(trips)
		for batch := range slicesChunk(trips, 50) {
			r.get(api + "/live?once=1&trip_ids=" + strings.Join(batch, ","))
		}
		r.get(api + "/updates")
		var lines []string
		for _, route := range bundle.Routes {
			lines = append(lines, url.QueryEscape(route.RouteShortName))
		}
		r.get(api + "/updates?routes=" + strings.Join(lines[:min(20, len(lines))], ","))
		r.get(api + "/route-changes")
		r.get("/" + city.Slug + "/")
	}
	r.get("/sitemap.xml")
	if adminToken == "" {
		return
	}
	flushStats(r.t, built)
	r.do(http.MethodPost, "/api/admin/login", `{"token":"`+adminToken+`"}`, "Content-Type", "application/json")
	r.get("/api/admin/site")
	for _, city := range built {
		r.get("/api/admin/overview?city=" + city.Slug)
		r.get("/api/admin/calendar?city=" + city.Slug)
	}
}

// Every route the app registers, so one dropped in a move shows even when nothing requests it.
func (r *recorder) routes() {
	seen := map[string]bool{}
	var lines []string
	for _, route := range r.app.GetRoutes(true) {
		line := route.Method + " " + route.Path
		if route.Method == http.MethodHead || seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
	}
	sort.Strings(lines)
	if err := os.WriteFile(filepath.Join(r.out, "routes.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// The flusher and a poll can fall on the same instant; flushed here, the admin page reads every count.
func flushStats(t *testing.T, built []*cities.City) {
	for _, city := range built {
		if err := city.FlushStats(); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *recorder) walkWrites(built []*cities.City, adminToken string) {
	jsonBody := []string{"Content-Type", "application/json"}
	slug := built[len(built)-1].Slug
	api := "/api/" + slug
	r.do(http.MethodPost, api+"/stats/event", `{"metric":"pwa_install","key":"appinstalled"}`, jsonBody...)
	r.do(http.MethodPost, api+"/stats/event", `{"metric":"nope","key":"x"}`, jsonBody...)

	r.walkSync()

	if adminToken == "" {
		return
	}
	r.do(http.MethodPost, "/api/admin/login", `{"token":"`+adminToken+`"}`, jsonBody...)
	day := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	r.do(http.MethodPut, "/api/admin/calendar/day?city="+slug, `{"date":"`+day+`","schedule":"saturday"}`, jsonBody...)
	r.get("/api/" + slug + "/network")
	r.do(http.MethodPut, "/api/admin/calendar/day?city="+slug, `{"date":"`+day+`","schedule":""}`, jsonBody...)
	r.do(http.MethodPut, "/api/admin/cities/"+slug, `{"enabled":false}`, jsonBody...)
	r.get("/api/" + slug + "/network/version")
	r.get("/api/cities")
	r.do(http.MethodPut, "/api/admin/cities/"+slug, `{"enabled":true}`, jsonBody...)
	r.do(http.MethodPut, "/api/admin/cities/nope", `{"enabled":true}`, jsonBody...)
	r.do(http.MethodPost, "/api/admin/logout", "")
}

func (r *recorder) walkCity(slug string) {
	api := "/api/" + slug
	var bundle bundleShape
	if err := json.Unmarshal(r.get(api+"/network"), &bundle); err != nil {
		r.t.Fatalf("%s network: %v", slug, err)
	}
	r.get(api+"/network", "Accept-Encoding", "gzip")
	r.get(api + "/network/version")

	var trips []string
	for _, route := range bundle.Routes {
		for _, trip := range route.Trips {
			trips = append(trips, trip.TripID)
		}
	}
	sort.Strings(trips)
	stops := bundle.Stops
	sort.Slice(stops, func(i, j int) bool { return stops[i].StopID < stops[j].StopID })

	shapes := strings.Join(trips[:min(6, len(trips))], ",")
	r.get(api + "/shapes?shape_ids=" + shapes)
	r.get(api + "/shapes?format=polyline&shape_ids=" + shapes)
	for batch := range slicesChunk(trips, 25) {
		r.get(api + "/live?once=1&trip_ids=" + strings.Join(batch, ","))
	}
	var sampled []string
	for i := 0; i < len(stops); i += 10 {
		sampled = append(sampled, strconv.Itoa(stops[i].StopID))
	}
	for batch := range slicesChunk(sampled, 40) {
		r.get(api + "/live?once=1&stop_ids=" + strings.Join(batch, ","))
	}
	if len(bundle.Routes) > 0 {
		r.get(api + "/live?once=1&route_ids=" + strconv.Itoa(bundle.Routes[0].RouteID))
	}
	r.get(api + "/live?once=1")

	if n := len(stops); n > 4 {
		pairs := [][2]int{{0, n / 2}, {n / 3, 2 * n / 3}, {n / 4, 3 * n / 4}}
		for i, p := range pairs {
			from, to := stops[p[0]], stops[p[1]]
			q := fmt.Sprintf("from_lat=%f&from_lng=%f&to_lat=%f&to_lng=%f", from.Lat, from.Lon, to.Lat, to.Lon)
			r.get(api + "/plan?" + q)
			switch i {
			case 1:
				r.get(api + "/plan?" + q + "&time=2026-10-20T08:30&arrive_by=1")
			case 2:
				r.get(api + "/plan?" + q + "&last=true")
			}
		}
		word, _, _ := strings.Cut(stops[n/2].StopName, " ")
		r.get(api + "/places?limit=5&q=" + url.QueryEscape(word))
		r.get(fmt.Sprintf("%s/places?lat=%f&lon=%f", api, stops[n/2].Lat, stops[n/2].Lon))
	}
	r.get(api + "/route-changes")
	r.get(api + "/updates")
	var areas []struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(r.get(api+"/destinations"), &areas); err != nil {
		r.t.Fatalf("%s destinations: %v", slug, err)
	}
	for _, a := range areas {
		r.get(api + "/destinations/" + a.Slug)
	}
	r.get(api + "/destinations/nope")

	page := "/" + slug
	r.get(page + "/destinatii")
	r.get(page + "/destinations")
	for _, a := range areas {
		r.get(page + "/spre/" + a.Slug)
	}
	if len(areas) > 0 {
		r.get(page + "/spre/" + areas[0].Slug + "?lang=en")
		r.get(page + "/to/" + areas[0].Slug + "?lang=en")
	}
	r.get(page + "/spre/nope")
	r.get(page + "/")
	r.get(page + "/?lang=en")
	r.get(page + "/plan")
	r.get(page + "/plan?name=" + url.QueryEscape("Piața <b>Unirii</b>") + "&lat=46.77&lon=23.59")
	r.get(page + "/nope")
	for i, route := range bundle.Routes {
		if i == 3 {
			break
		}
		r.get(fmt.Sprintf("%s/route/%d/0", page, route.RouteID))
		r.get(fmt.Sprintf("%s/route/%d/1", page, route.RouteID))
		r.get(page + "/linia/" + route.Slug)
		r.get(page + "/linia/" + route.Slug + "/intors")
	}
	for i := 0; i < min(3, len(stops)); i++ {
		r.get(fmt.Sprintf("%s/stop/%d", page, stops[i*len(stops)/3].StopID))
		r.get(page + "/statia/" + stops[i*len(stops)/3].Slug)
	}
	if len(stops) > 0 {
		r.get(fmt.Sprintf("%s/stop/%d?lang=en", page, stops[0].StopID))
		r.get(page + "/statia/" + stops[0].Slug + "?lang=en")
	}
	r.get(page + "/stop/999999999")
	r.get(page + "/route/999999999/0")
	r.get(page + "/statia/nope")
	r.get(page + "/linia/nope")
}

func slicesChunk(s []string, n int) func(func([]string) bool) {
	return func(yield func([]string) bool) {
		for i := 0; i < len(s); i += n {
			if !yield(s[i:min(i+n, len(s))]) {
				return
			}
		}
	}
}

func (r *recorder) get(target string, header ...string) []byte {
	return r.do(http.MethodGet, target, "", header...)
}

var (
	etagLine     = regexp.MustCompile(`(?m)^Etag: .*$`)
	unsafeName   = regexp.MustCompile(`[^A-Za-z0-9._=-]+`)
	cookieValue  = regexp.MustCompile(`^([^=]+)=[^;]*`)
	skippedHeads = map[string]bool{"Date": true}
)

func (r *recorder) do(method, target, body string, header ...string) []byte {
	r.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("User-Agent", "Mozilla/5.0 (parity)")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	if r.cookie != "" {
		req.Header.Set("Cookie", r.cookie)
	}
	resp, err := r.app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		r.t.Fatalf("%s %s: %v", method, target, err)
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		r.t.Fatalf("%s %s: %v", method, target, err)
	}
	synctest.Wait()

	plain := raw
	if resp.Header.Get("Content-Encoding") == "gzip" {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			r.t.Fatalf("%s %s: %v", method, target, err)
		}
		if plain, err = io.ReadAll(zr); err != nil {
			r.t.Fatalf("%s %s: %v", method, target, err)
		}
	}
	for _, c := range resp.Header.Values("Set-Cookie") {
		if name, value, ok := strings.Cut(strings.SplitN(c, ";", 2)[0], "="); ok {
			if value == "" {
				r.cookie = ""
			} else {
				r.cookie = name + "=" + value
			}
		}
	}

	var text strings.Builder
	fmt.Fprintf(&text, "%s %s\n", method, target)
	for i := 0; i+1 < len(header); i += 2 {
		fmt.Fprintf(&text, "> %s: %s\n", header[i], header[i+1])
	}
	fmt.Fprintf(&text, "%d\n", resp.StatusCode)
	volatileBody := strings.HasPrefix(target, "/api/admin/overview") || strings.HasPrefix(target, "/api/admin/site")
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		if !skippedHeads[k] && !((k == "Content-Length" || k == "Etag") && (volatileBody || resp.Header.Get("Content-Encoding") != "")) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range resp.Header.Values(k) {
			if k == "Set-Cookie" {
				v = cookieValue.ReplaceAllString(v, "$1=<value>")
			}
			fmt.Fprintf(&text, "%s: %s\n", k, v)
		}
	}
	text.WriteString("\n")
	if volatileBody {
		plain = maskVolatile(r.t, plain)
	}
	if strings.HasPrefix(target, "/api/sync") {
		r.maskSyncValues(plain)
	}
	var pretty bytes.Buffer
	if strings.Contains(resp.Header.Get("Content-Type"), "json") && json.Indent(&pretty, plain, "", " ") == nil {
		text.Write(pretty.Bytes())
	} else {
		text.Write(plain)
	}
	text.WriteString("\n")
	recorded := text.String()
	// Only sync answers hold the codes; anywhere else a six-digit code can match part of a coordinate.
	if strings.HasPrefix(target, "/api/sync") {
		if masked := r.mask(recorded); masked != recorded {
			recorded = etagLine.ReplaceAllString(masked, "Etag: <masked>")
		}
		for _, m := range r.masks {
			target = strings.ReplaceAll(target, m[0], m[1])
			for i := range header {
				header[i] = strings.ReplaceAll(header[i], m[0], m[1])
			}
		}
	}

	if err := os.WriteFile(filepath.Join(r.out, r.fileName(method, target, header)), []byte(recorded), 0o644); err != nil {
		r.t.Fatal(err)
	}
	return plain
}

// What the process itself measures: memory, disk, start time.
var volatileKeys = map[string]bool{"started_at": true, "heap_bytes": true, "db_bytes": true, "disk_free": true, "disk_total": true}

func maskVolatile(t *testing.T, body []byte) []byte {
	var doc map[string]any
	if json.Unmarshal(body, &doc) != nil {
		return body
	}
	var mask func(v any)
	mask = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, inner := range v {
				if volatileKeys[k] {
					v[k] = "<volatile>"
				} else {
					mask(inner)
				}
			}
		case []any:
			for _, inner := range v {
				mask(inner)
			}
		}
	}
	mask(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (r *recorder) mask(text string) string {
	for _, m := range r.masks {
		text = strings.ReplaceAll(text, m[0], m[1])
	}
	return text
}

func (r *recorder) fileName(method, target string, header []string) string {
	name := strings.Trim(unsafeName.ReplaceAllString(r.prefix+method+" "+target+" "+strings.Join(header, " "), "_"), "_")
	if len(name) > 120 {
		sum := sha1.Sum([]byte(name))
		name = name[:100] + "-" + hex.EncodeToString(sum[:6])
	}
	r.names[name]++
	if n := r.names[name]; n > 1 {
		name += "~" + strconv.Itoa(n)
	}
	return name + ".txt"
}
