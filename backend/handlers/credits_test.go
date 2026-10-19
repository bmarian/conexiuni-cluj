package handlers

import (
	"conexiuni-cluj/sources"
	"conexiuni-cluj/sources/gtfsrt"
	"conexiuni-cluj/sources/tranzy"
	"testing"
)

func TestCreditsListOnlySourcesInUse(t *testing.T) {
	city := newTestCity(t)
	if got := city.credits(); len(got) != 0 {
		t.Fatalf("no sources: got %v", got)
	}

	city.Realtime = gtfsrt.New("", nil)
	city.Learning = city.Realtime
	if got := city.credits(); len(got) != 0 {
		t.Fatalf("operator's own realtime feed: got %v", got)
	}

	source := tranzy.New(nil)
	city.Static, city.Realtime, city.Learning = source, source, tranzy.New(nil)
	want := []sources.Credit{{Name: "tranzy.ai", URL: "https://tranzy.ai/"}}
	if got := city.credits(); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("tranzy city: got %v, want %v", got, want)
	}
}
