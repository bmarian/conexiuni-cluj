package handlers

import (
	"bufio"
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"os"
	"testing"
	"time"
)

type replayNoPersist struct{}

func (replayNoPersist) Save(string, int, time.Time) error   { return nil }
func (replayNoPersist) Load(string) (int, time.Time, error) { return 0, time.Time{}, nil }

// Feeds the scoring harness. REPLAY_FILE=... REPLAY_DB=... REPLAY_OUT=... go test -run TestReplayLiveRows ./handlers/
func TestReplayLiveRows(t *testing.T) {
	file, dbPath, outPath := os.Getenv("REPLAY_FILE"), os.Getenv("REPLAY_DB"), os.Getenv("REPLAY_OUT")
	if file == "" || dbPath == "" || outPath == "" {
		t.Skip("REPLAY_FILE, REPLAY_DB and REPLAY_OUT not set")
	}
	if err := database.Connect(dbPath); err != nil {
		t.Fatal(err)
	}
	if err := database.InitSchemas(); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Close()
	loc, _ := time.LoadLocation("Europe/Bucharest")
	// What the learners would have derived from the snapshot's samples by now.
	if n, err := recomputeScheduleDelays(loc, time.Now()); err != nil {
		t.Logf("schedule delays: %v", err)
	} else {
		t.Logf("schedule delays: %d profiles", n)
	}
	resetProfileSnapshots()
	resetTurnaroundCache()
	liveDeps.tranzy = tranzy.NewClient("http://127.0.0.1:1", "none", "2", time.Second, 1, 1, replayNoPersist{})
	liveDeps.ctp = ctpcj.NewClient("http://127.0.0.1:1", time.Second)
	liveDeps.cacheTimes = models.CacheTimes{TranzyCacheShelfLife: 365 * 24 * time.Hour, TimetableCacheShelfLife: 365 * 24 * time.Hour}
	livePollInterval = func() time.Duration { return 15 * time.Second }

	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	w := bufio.NewWriterSize(out, 1<<20)
	defer w.Flush()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 64<<20)
	previous := make(map[int]models.Vehicle)
	frames := 0
	for scanner.Scan() {
		var frame struct {
			T        float64          `json:"t"`
			Vehicles []models.Vehicle `json:"vehicles"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil || len(frame.Vehicles) == 0 {
			continue
		}
		at := time.Unix(int64(frame.T), 0).In(loc)
		liveClock = func() time.Time { return at }
		refreshLiveTrips(false)
		for i := range frame.Vehicles {
			v := &frame.Vehicles[i]
			v.TripID = NormalizeTripID(v.TripID)
			prev, had := previous[v.ID]
			carryTripChange(v, prev, had)
			previous[v.ID] = *v
		}
		table := RecomputeLive(frame.Vehicles, at)
		liveTrips.RLock()
		trips := liveTrips.byTrip
		liveTrips.RUnlock()
		type entry struct {
			At        int64  `json:"at"`
			Live      bool   `json:"live"`
			VehicleID int    `json:"vehicle_id,omitempty"`
			Kind      string `json:"kind,omitempty"`
		}
		type row struct {
			TripID  string  `json:"trip_id"`
			StopID  int     `json:"stop_id"`
			Entries []entry `json:"entries"`
		}
		kinds := map[predictions.EtaKind]string{predictions.EtaOnTrip: "ontrip", predictions.EtaStanding: "standing", predictions.EtaFeeder: "feeder"}
		rec := struct {
			T    float64          `json:"t"`
			Rows map[string][]row `json:"rows"`
		}{T: frame.T, Rows: map[string][]row{}}
		for tripID, lt := range trips {
			if lt.trip.Turnaround == nil || !lt.trip.Turnaround.Through {
				continue
			}
			var rows []row
			for _, r := range table.rowsByTrip[tripID] {
				out := row{TripID: r.TripID, StopID: r.StopID, Entries: []entry{}}
				for _, e := range r.Entries {
					kind := ""
					if e.Live {
						kind = kinds[e.kind]
					}
					out.Entries = append(out.Entries, entry{At: e.At, Live: e.Live, VehicleID: e.VehicleID, Kind: kind})
				}
				rows = append(rows, out)
			}
			rec.Rows[tripID] = rows
		}
		line, _ := json.Marshal(rec)
		w.Write(line)
		w.WriteByte('\n')
		frames++
	}
	liveClock = time.Now
	t.Logf("replayed %d frames", frames)
}
