package botosani

import (
	"os"
	"testing"
)

func fixturePage(t *testing.T) ([]pageLine, string) {
	t.Helper()
	page, err := os.ReadFile("testdata/trasee.html")
	if err != nil {
		t.Fatal(err)
	}
	lines, program, err := parsePage(page, "https://eltransbt.ro/trasee")
	if err != nil {
		t.Fatal(err)
	}
	return lines, program
}

func TestParsePageReadsEachLineWithItsMapAndRuns(t *testing.T) {
	lines, program := fixturePage(t)
	if program != "https://eltransbt.ro/wp-content/uploads/2026/10/Program-de-circulat%CC%A6ie-autobuze.pdf" {
		t.Fatalf("program %q", program)
	}
	if len(lines) != 2 || lines[0].short != "BT01" || lines[1].short != "BT02" {
		t.Fatalf("lines %+v", lines)
	}
	want := []struct {
		markers int
		runs    map[string]int
	}{
		{25, map[string]int{"TUR": 13, "RETUR": 13}},
		{22, map[string]int{"TUR": 11, "RETUR": 11, "CURSA SCURTĂ": 12}},
	}
	for i, line := range lines {
		if len(line.markers) != want[i].markers || len(line.runs) != len(want[i].runs) {
			t.Fatalf("%s: %d markers, %d runs", line.short, len(line.markers), len(line.runs))
		}
		for _, r := range line.runs {
			if len(r.names) != want[i].runs[r.label] {
				t.Errorf("%s %s: %q", line.short, r.label, r.names)
			}
		}
	}
	if r := lines[0].runs[0]; r.names[0] != "Luceafărul" || r.names[len(r.names)-1] != "Bariera Cătămărăști" {
		t.Errorf("BT01 TUR %q", r.names)
	}
}

func TestMarkerNamesLoseTheirSide(t *testing.T) {
	lines, _ := fixturePage(t)
	names := map[string]string{}
	for _, m := range lines[0].markers {
		names[m.id] = m.name
	}
	for id, want := range map[string]string{"9": "Luceafărul", "17": "Școala 11", "28": "Electroalfa", "34": "Barieră Cătămărăști"} {
		if names[id] != want {
			t.Errorf("marker %s is %q, want %q", id, names[id], want)
		}
	}
	if nameKey("Electro Alfa") != nameKey(names["28"]) || nameKey("Bariera Cătămărăști") != nameKey(names["34"]) {
		t.Error("the page's spelling does not match the map's")
	}
}
