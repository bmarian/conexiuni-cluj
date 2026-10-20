package admin

import (
	"conexiuni-cluj/database"
	"testing"
	"time"
)

func TestErrorHistogramSummarize(t *testing.T) {
	// 6 on time, 2 a minute and a half late, 2 two minutes early.
	h := errorHistogram{0: 3, -1: 3, 3: 2, -4: 2}
	s := h.summarize("x")
	if s.N != 10 {
		t.Fatalf("N = %d", s.N)
	}
	if s.Within1 != 0.6 {
		t.Errorf("Within1 = %v, want 0.6", s.Within1)
	}
	if s.MedianAbs != 15 {
		t.Errorf("MedianAbs = %d, want 15", s.MedianAbs)
	}
	if s.P90Abs != 105 {
		t.Errorf("P90Abs = %d, want 105", s.P90Abs)
	}
	if (errorHistogram{}).summarize("empty").N != 0 {
		t.Error("empty histogram has samples")
	}
}

func TestProblemLogGroupsRepeats(t *testing.T) {
	p := &problemLog{}
	_, _ = p.Write([]byte("2026/10/14 10:00:00 live hub: poll failed interval=10s subs=3 err=timeout\n"))
	_, _ = p.Write([]byte("2026/10/14 10:00:10 live hub: poll failed interval=12s subs=4 err=timeout\n"))
	_, _ = p.Write([]byte("2026/10/14 10:00:11 segment travel: snapshot vehicles=10 sample_errors=0 profile_errors=0\n"))
	got := p.Recent(time.Time{})
	if len(got) != 1 {
		t.Fatalf("want 1 grouped line, got %+v", got)
	}
	if got[0].Count != 2 || got[0].Text != "live hub: poll failed interval=12s subs=4 err=timeout" {
		t.Errorf("unexpected line %+v", got[0])
	}
}

func TestTimetableAccuracyOverAWeek(t *testing.T) {
	eng := testCity(t, "test")
	// Other tests may leave today's unflushed counts behind.
	before, err := accuracySection(eng, time.Now())
	if err != nil {
		t.Fatalf("accuracy: %v", err)
	}
	for _, ago := range []int{1, 2, 8} {
		date := database.StatsDaysAgo(ago)
		for _, key := range []string{"ours|0", "ctp|2"} {
			if _, err := eng.DB.Exec(`INSERT INTO stats_daily (date, metric, key, count) VALUES (?, 'timetable_error', ?, 4)`, date, key); err != nil {
				t.Fatalf("stat: %v", err)
			}
		}
	}
	a, err := accuracySection(eng, time.Now())
	if err != nil {
		t.Fatalf("accuracy: %v", err)
	}
	if len(a.Timetable) != 2 {
		t.Fatalf("%d timetable rows", len(a.Timetable))
	}
	ours, ctp := a.Timetable[0], a.Timetable[1]
	if ours.N-before.Timetable[0].N != 8 || ctp.N-before.Timetable[1].N != 8 || ctp.MedianAbs != 75 {
		t.Fatalf("ours %+v, ctp %+v", ours, ctp)
	}
}
