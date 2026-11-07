package albaiulia

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/sitegtfs"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestIndex(t *testing.T) {
	index := `<div class='actualizare_orar'>Orarul a fost actualizat la: <strong>30-10-2026</strong>.</div>` +
		`<p class='linii_box'><a href='program.php?linie=101R_L1R'>101R</a></p><p class='linii_box'><a href='program.php?linie=101T_L1T'>101T</a></p>` +
		`<a href='program.php'>Program</a><a href='presa.php?id=73'>Programul autobuzelor 1 Mai 2026</a><a href='program.php?linie=101R_L1R'>101R</a>`
	links, updated, err := parseIndex([]byte(index), "https://www.stpalba.ro/program.php")
	if err != nil {
		t.Fatal(err)
	}
	want := []link{
		{url: "https://www.stpalba.ro/program.php?linie=101R_L1R", code: "101R_L1R", label: "101R"},
		{url: "https://www.stpalba.ro/program.php?linie=101T_L1T", code: "101T_L1T", label: "101T"},
	}
	if updated != "30-10-2026" || !slices.Equal(links, want) {
		t.Fatalf("got %q %+v", updated, links)
	}
}

func TestPage(t *testing.T) {
	data, err := os.ReadFile("testdata/505_SBG.html")
	if err != nil {
		t.Fatal(err)
	}
	// One stop the map leaves out, as it does on some of STP's pages.
	data = []byte(strings.Replace(string(data), "['Straja T',", "['skipped',", 1))
	p, err := parsePage(data, link{code: "505_SBG", label: "505"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Stops) != 22 || len(p.Minutes) != 22 {
		t.Fatalf("%d stops, %d minutes", len(p.Stops), len(p.Minutes))
	}
	first, last := p.Stops[0], p.Stops[21]
	if first.ID != "1000329" || first.Name != "Autogara STP" || !first.Placed || last.Name != "Ghirbom T" || p.Minutes[21] != 49 {
		t.Fatalf("ends %+v %+v, last minute %d", first, last, p.Minutes[21])
	}
	for _, s := range p.Stops {
		if s.Placed == (s.Name == "Straja T") {
			t.Fatalf("%s placed %v", s.Name, s.Placed)
		}
	}
	want := []sitegtfs.Departure{
		{Day: calendar.Weekday, Minute: 7*60 + 40}, {Day: calendar.Saturday, Minute: 7*60 + 40}, {Day: calendar.Sunday, Minute: 7*60 + 40},
		{Day: calendar.Weekday, Minute: 11*60 + 15, Only: sitegtfs.Only(time.Tuesday)},
		{Day: calendar.Saturday, Minute: 19*60 + 30}, {Day: calendar.Sunday, Minute: 19*60 + 30},
		{Day: calendar.Weekday, Minute: 23*60 + 30},
	}
	if !slices.Equal(p.Departures, want) {
		t.Fatalf("departures %+v", p.Departures)
	}
}
