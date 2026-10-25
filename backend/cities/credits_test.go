package cities

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/gtfs"
	"conexiuni-cluj/feeds/gtfsrt"
	"conexiuni-cluj/feeds/tranzy"
	"testing"
)

func TestCreditsListOnlySourcesInUse(t *testing.T) {
	eng := newNamedTestCity(t, "test", nil)
	if got := eng.credits(); len(got) != 0 {
		t.Fatalf("no sources: got %v", got)
	}

	eng.Realtime = gtfsrt.New("", nil)
	eng.Learning = eng.Realtime
	if got := eng.credits(); len(got) != 0 {
		t.Fatalf("operator's own realtime feed: got %v", got)
	}

	source := tranzy.New(nil)
	eng.Static, eng.Realtime, eng.Learning = source, source, tranzy.New(nil)
	want := []feeds.Credit{{Name: "tranzy.ai", URL: "https://tranzy.ai/"}}
	if got := eng.credits(); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("tranzy city: got %v, want %v", got, want)
	}

	author := feeds.Credit{Name: "volunteer", URL: "https://example.org/"}
	eng.Static, eng.Realtime, eng.Learning = feeds.WithCredit(gtfs.New("", 0, nil), author), nil, nil
	if got := eng.credits(); len(got) != 1 || got[0] != author {
		t.Fatalf("credited GTFS: got %v, want %v", got, author)
	}
}
