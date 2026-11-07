package slobozia

import (
	"conexiuni-cluj/feeds/sitegtfs"
	"os"
	"slices"
	"strings"
	"testing"
)

func bundleFixture(t *testing.T, edit func(string) string) (map[string]sitegtfs.Stop, map[string]route, []route) {
	t.Helper()
	data, err := os.ReadFile("testdata/busData.js")
	if err != nil {
		t.Fatal(err)
	}
	stops, routes, err := parseBundle([]byte(edit(string(data))))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]route{}
	for _, r := range routes {
		byID[r.id] = r
	}
	return stops, byID, routes
}

func hm(h, m int) int { return h*60 + m }

func TestBundle(t *testing.T) {
	stops, routes, all := bundleFixture(t, func(s string) string { return s })
	if len(stops) != 35 || len(all) != 11 {
		t.Fatalf("%d stops, %d routes", len(stops), len(all))
	}
	if s := stops["piata_garii"]; s.Name != "Piața Gării" || s.At.Lat != 44.564469 || s.At.Lon != 27.350747 || !s.Placed {
		t.Fatalf("stop %+v", s)
	}
	out := routes["L1_TUR"]
	// The table's 34 weekday runs, 16:00 among them although the times written into the route lack it.
	if out.line != "1" || len(out.stops) != 15 || out.dwell != 0 || len(out.leaves[weekdays]) != 34 || !slices.Contains(out.leaves[weekdays], hm(16, 0)) || out.minutes[14] != 19 {
		t.Fatalf("L1_TUR %+v", out)
	}
	if back := routes["L1_RETUR"]; back.leaves[weekdays][0] != hm(5, 0) || back.leaves[weekends][0] != hm(5, 30) {
		t.Fatalf("L1_RETUR leaves %v", back.leaves)
	}
	loop := routes["L3A_TUR"]
	if loop.dwell != 6 || loop.rideBack != 17 {
		t.Fatalf("L3A_TUR waits at %d, rides back %d", loop.dwell, loop.rideBack)
	}
	if want := []int{hm(8, 20), hm(9, 20), hm(10, 20), hm(11, 20), hm(15, 15), hm(16, 15), hm(18, 7), hm(19, 7)}; !slices.Equal(loop.leaves[weekdays], want) {
		t.Errorf("L3A_TUR weekdays %v", loop.leaves[weekdays])
	}
	if want := []int{hm(8, 40), hm(9, 40), hm(10, 40), hm(11, 40), hm(15, 30), hm(16, 30), hm(18, 20), hm(19, 20)}; !slices.Equal(loop.turns[weekdays], want) {
		t.Errorf("L3A_TUR turns %v", loop.turns[weekdays])
	}
	if want := []int{hm(7, 20), hm(11, 0), hm(13, 45), hm(16, 25), hm(19, 5)}; !slices.Equal(loop.leaves[weekends], want) {
		t.Errorf("L3A_TUR weekends %v", loop.leaves[weekends])
	}
	if detour := routes["L3A_ELEVI_TUR"]; !slices.Equal(detour.leaves[weekdays], []int{hm(7, 13), hm(12, 7), hm(13, 7)}) || len(detour.leaves[weekends]) != 0 {
		t.Errorf("L3A_ELEVI_TUR %v", detour.leaves)
	}
}

func TestTimesWrittenIntoARoute(t *testing.T) {
	_, routes, _ := bundleFixture(t, func(s string) string { return strings.Replace(s, "L1_TUR:g(E),", "", 1) })
	if got := routes["L1_TUR"].leaves[weekdays]; len(got) != 33 || slices.Contains(got, hm(16, 0)) || got[0] != hm(4, 30) {
		t.Fatalf("weekdays without a table %v", got)
	}
}
