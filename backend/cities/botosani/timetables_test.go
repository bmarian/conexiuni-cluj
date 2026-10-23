package botosani

import (
	"conexiuni-cluj/calendar"
	"os"
	"slices"
	"testing"
)

func fixtureProgram(t *testing.T) map[string]*lineProgram {
	t.Helper()
	data, err := os.ReadFile("testdata/program.pdf")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := readProgram(data)
	if err != nil {
		t.Fatal(err)
	}
	return parseProgram(rows)
}

func counts(buses [][]departure, variant bool) []int {
	out := make([]int, len(buses))
	for i, bus := range buses {
		for _, d := range bus {
			if d.variant == variant {
				out[i]++
			}
		}
	}
	return out
}

func TestParseProgramReadsEveryBusOfEveryDay(t *testing.T) {
	programs := fixtureProgram(t)
	if len(programs) != 2 || programs["BT01"] == nil || programs["BT02"] == nil {
		t.Fatalf("programs %v", programs)
	}
	bt01, bt02 := programs["BT01"].from["Luceafărul"], programs["BT02"].from["George Enescu"]
	for _, c := range []struct {
		buses   [][]departure
		variant bool
		want    []int
	}{
		{bt01[calendar.Weekday], false, []int{21, 19, 20, 20, 17}},
		{bt01[calendar.Saturday], false, []int{15, 15, 15, 15}},
		{bt01[calendar.Sunday], false, []int{14, 14, 14, 13}},
		{bt02[calendar.Weekday], false, []int{22, 20, 20, 21}},
		{bt02[calendar.Weekday], true, []int{3, 3, 4, 3}},
		{bt02[calendar.Saturday], true, []int{11, 11, 11}},
		{bt02[calendar.Sunday], true, []int{15, 15, 15}},
	} {
		if got := counts(c.buses, c.variant); !slices.Equal(got, c.want) {
			t.Errorf("got %v departures per bus, want %v", got, c.want)
		}
	}
	if first, last := bt01[calendar.Weekday][0][0], bt01[calendar.Weekday][0][20]; first.minute != 4*60+50 || last.minute != 23*60 {
		t.Errorf("BT01's first bus runs %+v to %+v", first, last)
	}
	// Printed "14.10".
	if !slices.Contains(bt02[calendar.Weekday][3], departure{minute: 14*60 + 10}) {
		t.Errorf("BT02's fourth bus %+v", bt02[calendar.Weekday][3])
	}
	if programs["BT02"].variant != "A" || programs["BT01"].variant != "" {
		t.Errorf("variants %q %q", programs["BT01"].variant, programs["BT02"].variant)
	}
}
