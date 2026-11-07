package ramnicuvalcea

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/sitegtfs"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func stops(ids ...string) []sitegtfs.Stop {
	out := make([]sitegtfs.Stop, len(ids))
	for i, id := range ids {
		out[i] = sitegtfs.Stop{ID: id, Name: id}
	}
	return out
}

// The run's times at its stops, in order, as the stop pages show them.
func ride(line string, way int, trip string, days []string, at ...any) []passing {
	var out []passing
	for i := 0; i < len(at); i += 2 {
		out = append(out, passing{stop: at[i].(string), line: line, way: way, days: days, trip: trip, minute: at[i+1].(int)})
	}
	return out
}

func TestLines(t *testing.T) {
	weekday, weekend := []string{calendar.Weekday}, []string{calendar.Saturday, calendar.Sunday}
	pages := map[string]ways{"7": {stops("A", "B", "C", "D"), stops("D", "C", "B", "A")}, "8": {stops("X", "Y"), nil}}
	var passings []passing
	// Line 8 leaves every hour from 05:00 to 23:00, so the network pauses at night.
	for h := 5; h <= 23; h++ {
		passings = append(passings, ride("8", 0, fmt.Sprintf("8_%d_0", h), weekday, "X", h*60, "Y", h*60+4)...)
	}
	for _, p := range [][]passing{
		ride("7", 0, "7_1_0", weekday, "B", 6*60, "C", 6*60+5, "D", 6*60+12),
		ride("7", 0, "7_2_0", weekend, "B", 7*60, "C", 7*60+5, "D", 7*60+12),
		// Past midnight on the way.
		ride("7", 0, "7_3_0", weekday, "B", 23*60+55, "C", 0, "D", 7),
		// A longer run from the line's first stop.
		ride("7", 0, "8_1_0", weekday, "A", 5*60+50, "B", 6*60, "C", 6*60+5, "D", 6*60+12),
		// The night's last run, after midnight.
		ride("7", 1, "7_1_1", weekday, "D", 30, "C", 37, "B", 42),
		ride("7", 1, "7_2_1", weekday, "D", 6*60+20, "C", 6*60+27, "B", 6*60+32),
		// A line the index does not list.
		ride("99", 0, "99_1_0", weekday, "A", 8*60, "B", 8*60+3),
	} {
		passings = append(passings, p...)
	}
	var got []string
	for _, l := range lines([]link{{name: "7"}, {name: "8"}}, pages, passings) {
		for w, r := range l.Ways {
			if r == nil || l.Name != "7" {
				continue
			}
			var ids, deps []string
			for _, s := range r.Stops {
				ids = append(ids, s.ID)
			}
			for _, d := range r.Departures {
				deps = append(deps, fmt.Sprintf("%s %d", d.Day, d.Minute))
			}
			got = append(got, fmt.Sprintf("%s/%d %s %s %v [%s]", l.Name, w, r.ID, strings.Join(ids, ""), r.Minutes, strings.Join(deps, ", ")))
		}
	}
	want := []string{
		"7/0 7-0 ABCD [0 10 15 22] [weekday 350]",
		"7/0 7-1 BCD [0 5 12] [weekday 360, saturday 420, sunday 420, weekday 1435]",
		"7/1 7-2 DCB [0 7 12] [weekday 380, weekday 1470]",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

func TestStationName(t *testing.T) {
	for name, want := range map[string]string{
		"Gara Tur": "Gara", "Gara Retur": "Gara", "Mihai Viteazul Piata  Tur": "Mihai Viteazul Piata", "Str. Republicii Bl.R24.Retur": "Str. Republicii Bl.R24",
		"Str. Someș (Vidra)Tur": "Str. Someș (Vidra)", "Uzina G ": "Uzina G", "Toamnei 1( Cimitir Retur)": "Toamnei 1( Cimitir Retur)", "Abator": "Abator", "Tur": "Tur",
	} {
		if got := stationName(name); got != want {
			t.Errorf("%q: %q", name, got)
		}
	}
}
