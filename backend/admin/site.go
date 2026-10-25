package admin

import (
	"cmp"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/database"
	"conexiuni-cluj/devicesync"
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"time"
)

var serverStartedAt = time.Now()

const diskFullWarning = 0.9

type citySummary struct {
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	Realtime    bool          `json:"realtime"`
	Watching    int           `json:"watching"`
	PeopleToday int           `json:"people_today"`
	Problems    []string      `json:"problems"`
	Errors      []errorDetail `json:"errors"`
}

type siteServer struct {
	StartedAt time.Time         `json:"started_at"`
	Revision  string            `json:"revision"`
	Modified  bool              `json:"modified"`
	HeapBytes uint64            `json:"heap_bytes"`
	DBBytes   int64             `json:"db_bytes"`
	DiskFree  uint64            `json:"disk_free"`
	DiskTotal uint64            `json:"disk_total"`
	Requests  int               `json:"requests"`
	Errors    int               `json:"errors"`
	Problems  []problemEntry    `json:"problems"`
	Sync      *devicesync.Stats `json:"sync,omitempty"`
}

type adminSite struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Problems    []string      `json:"problems"`
	Cities      []citySummary `json:"cities"`
	Server      siteServer    `json:"server"`
}

func buildSite(registry *cities.Registry, syncStats func() (devicesync.Stats, error), now time.Time) *adminSite {
	s := &adminSite{GeneratedAt: now, Cities: []citySummary{}, Server: serverInfo(now)}
	for _, city := range registry.Enabled() {
		summary, traffic := summarize(city, now)
		s.Cities = append(s.Cities, summary)
		s.Server.Requests += traffic.Requests
		s.Server.Errors += traffic.Errors
		s.Server.DBBytes += traffic.DBBytes
	}
	slices.SortFunc(s.Cities, func(a, b citySummary) int { return cmp.Compare(a.Name, b.Name) })
	if stats, err := syncStats(); err == nil {
		s.Server.Sync = &stats
	}
	s.Problems = siteProblems(s.Server, now)
	return s
}

func summarize(city *cities.City, now time.Time) (citySummary, adminTraffic) {
	st := city.Live.Status(now)
	n := nowCounts(st)
	s := citySummary{Slug: city.Slug, Name: city.Name, Realtime: city.Realtime != nil, Watching: n.People, Errors: []errorDetail{}}
	today := database.StatsDate(now)
	traffic, err := trafficSection(city, now)
	people, peopleErr := peopleByDay(city, today)
	if err = cmp.Or(err, peopleErr); err != nil {
		s.Problems = []string{fmt.Sprintf("Could not read: %v", err)}
		return s, traffic
	}
	s.PeopleToday, s.Errors = people[today], traffic.ErrorDetails
	quota, _ := quotaSection(city, st)
	s.Problems = cityProblems(city, st, n, quota.Quotas, traffic.Errors, archiveSection(city, st.Archive), now)
	return s, traffic
}

func serverInfo(now time.Time) siteServer {
	s := siteServer{StartedAt: serverStartedAt, Problems: []problemEntry{}}
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
	s.DiskFree, s.DiskTotal, _ = diskSpace(".")
	for _, l := range ProblemLog.Recent(now.Add(-24 * time.Hour)) {
		s.Problems = append(s.Problems, problemEntry{First: l.First, Last: l.Last, Count: l.Count, Text: l.Text})
	}
	return s
}

func siteProblems(s siteServer, now time.Time) []string {
	out := []string{}
	if s.DiskTotal > 0 {
		if used := 1 - float64(s.DiskFree)/float64(s.DiskTotal); used >= diskFullWarning {
			out = append(out, fmt.Sprintf("Disk %.0f%% full", used*100))
		}
	}
	for _, p := range s.Problems {
		if now.Sub(p.Last) < time.Hour {
			out = append(out, "Failures logged in the last hour")
			break
		}
	}
	return out
}
