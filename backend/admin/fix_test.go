package admin

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/live"
	"conexiuni-cluj/planner"
	"testing"
)

func TestFixListsKeepTheWorstFirst(t *testing.T) {
	city := testCity(t, "fix")
	insert := func(ago int, metric, key string, count int) {
		if _, err := city.DB.Exec(`INSERT INTO stats_daily (date, metric, key, count) VALUES (?, ?, ?, ?)`, database.StatsDaysAgo(ago), metric, key, count); err != nil {
			t.Fatalf("stat: %v", err)
		}
	}
	insert(1, planner.PlaceMissMetric, "iulius mal", 2)
	insert(2, planner.PlaceMissMetric, "iulius mall x", 1)
	insert(1, planner.PlaceMissMetric, "zorilor", 3)
	insert(adminFixDays, planner.PlaceMissMetric, "old", 9)
	// Line 1 is a little off often, line 2 far off rarely.
	insert(1, live.LineAccuracyMetric, "1|n", 100)
	insert(1, live.LineAccuracyMetric, "1|off", 30)
	insert(1, live.LineAccuracyMetric, "1|abs", 6000)
	insert(1, live.LineAccuracyMetric, "2|n", 10)
	insert(1, live.LineAccuracyMetric, "2|off", 8)
	insert(1, live.LineAccuracyMetric, "2|abs", 3000)
	insert(1, live.UnplacedMetric, "trip|5|101", 4)
	insert(2, live.UnplacedMetric, "trip|5|99", 1)
	insert(1, live.UnplacedMetric, "route|6|7", 40)
	insert(1, live.SeenMetric, "5", 10)
	insert(1, live.SeenMetric, "6", 400)

	f, err := fixSection(city)
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if len(f.Searches) != 2 || f.Searches[0].Query != "zorilor" || f.Searches[1].Query != "iulius mall x" {
		t.Errorf("searches %+v", f.Searches)
	}
	if len(f.Lines) != 2 || f.Lines[0].ID != 1 || f.Lines[0].AbsSec != 60 || f.Lines[1].AbsSec != 300 {
		t.Errorf("lines %+v", f.Lines)
	}
	if len(f.Buses) != 2 || f.Buses[0].ID != 5 || len(f.Buses[0].Buses) != 2 || f.Buses[0].Buses[0] != "99" || f.Buses[0].Count != 5 || f.Buses[0].Share != 0.5 || f.Buses[1].Share != 0.1 {
		t.Errorf("buses %+v", f.Buses)
	}
}
