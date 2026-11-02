package admin

import (
	"testing"
	"time"
)

func TestFailuresGoToTheirCity(t *testing.T) {
	saved := ProblemLog
	t.Cleanup(func() { ProblemLog = saved })
	ProblemLog = &problemLog{}
	now := time.Now()
	ProblemLog.add("targumures: live hub: poll failed", now)
	ProblemLog.add("sync: write failed", now)

	city := cityFailures("targumures", now)
	if len(city) != 1 || city[0].Text != "live hub: poll failed" {
		t.Fatalf("city failures = %+v", city)
	}
	if got := cityFailures("cluj", now); len(got) != 0 {
		t.Fatalf("another city's failure shown: %+v", got)
	}
	app := appFailures([]string{"cluj", "targumures"}, now)
	if len(app) != 1 || app[0].Text != "sync: write failed" {
		t.Fatalf("app failures = %+v", app)
	}
	if !failedLately(city, now) || failedLately(city, now.Add(recentFailure)) {
		t.Fatal("last-hour check")
	}
}
