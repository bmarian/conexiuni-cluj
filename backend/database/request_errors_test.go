package database

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRequestErrorsKeepTheLatestPathAcrossFlushes(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"), "", 5)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	now := time.Now()
	s.RecordRequestError("GET /stop/:id|500|boom", "/api/x/stop/2", now)
	s.RecordRequestError("GET /stop/:id|500|boom", "/api/x/stop/1", now.Add(-time.Minute))
	if err := s.FlushStats(); err != nil {
		t.Fatal(err)
	}
	s.RecordRequestError("GET /stop/:id|500|boom", "/api/x/stop/3", now.Add(time.Minute))
	if err := s.FlushStats(); err != nil {
		t.Fatal(err)
	}
	rows, err := s.RequestErrors(StatsDate(now))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Count != 3 || rows[0].Path != "/api/x/stop/3" || rows[0].LastAt != now.Add(time.Minute).Unix() {
		t.Fatalf("rows %+v", rows)
	}
}
