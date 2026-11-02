package admin

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDismissalLastsUntilTheProblemGoes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-dismissed.json")
	d := openDismissals(path)
	stale := func() []problem { return []problem{{Key: "stale-positions", Text: "Bus positions are 6 min old"}} }

	if err := d.set("cluj", "stale-positions", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := d.apply("cluj", stale()); !got[0].Dismissed {
		t.Fatal("dismissed problem shown")
	}
	if got := openDismissals(path).apply("cluj", stale()); !got[0].Dismissed {
		t.Fatal("dismissal lost on restart")
	}
	if got := d.apply("iasi", stale()); got[0].Dismissed {
		t.Fatal("dismissal leaked into another city")
	}

	d.apply("cluj", []problem{})
	if got := d.apply("cluj", stale()); got[0].Dismissed {
		t.Fatal("problem that came back stayed dismissed")
	}
}
