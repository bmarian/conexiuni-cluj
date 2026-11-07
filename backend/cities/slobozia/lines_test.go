package slobozia

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/sitegtfs"
	"slices"
	"testing"
)

func TestLines(t *testing.T) {
	stops, _, routes := bundleFixture(t, func(s string) string { return s })
	ways := map[string]*sitegtfs.Route{}
	dirs := map[string]int{}
	for _, l := range lines(stops, routes) {
		for dir, w := range l.Ways {
			if w != nil {
				ways[w.ID], dirs[w.ID] = w, dir
			}
		}
	}
	if len(ways) != 14 || dirs["L1_TUR"] != 0 || dirs["L1_RETUR"] != 1 {
		t.Fatalf("%d ways, directions %v", len(ways), dirs)
	}
	out, back := ways["L3A_TUR"], ways["L3A_RETUR"]
	if dirs["L3A_RETUR"] != 1 || len(out.Stops) != 7 || len(back.Stops) != 14 || out.Stops[6].Name != "1 Decembrie" || back.Stops[0].Name != "1 Decembrie" || back.Stops[13].Name != "Piața Gării" {
		t.Fatalf("3A split into %+v and %+v", out.Stops, back.Stops)
	}
	if !slices.Equal(out.Minutes, []int{0, 0, 2, 3, 5, 6, 9}) || !slices.Equal(back.Minutes, []int{0, 0, 1, 3, 4, 5, 6, 8, 9, 12, 14, 15, 17, 19}) {
		t.Fatalf("3A minutes %v and %v", out.Minutes, back.Minutes)
	}
	// 3B's rows ride back in 8 minutes where the bundle's stops after the first leg take 7.
	if got := ways["L3B_RETUR"].Minutes; got[1] != 1 || got[len(got)-1] != 8 {
		t.Fatalf("3B back minutes %v", got)
	}
	first := back.Departures[0]
	if first != (sitegtfs.Departure{Day: calendar.Weekday, Minute: hm(8, 40)}) || len(back.Departures) != 8+5*2 {
		t.Fatalf("3A back departures %+v", back.Departures)
	}
	saturdays := 0
	for _, d := range ways["L1_TUR"].Departures {
		if d.Day == calendar.Saturday {
			saturdays++
		}
	}
	if saturdays != 22 {
		t.Fatalf("%d Saturday departures on 1", saturdays)
	}
}
