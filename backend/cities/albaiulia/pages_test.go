package albaiulia

import (
	"conexiuni-cluj/calendar"
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
	if len(p.stops) != 22 || len(p.minutes) != 22 {
		t.Fatalf("%d stops, %d minutes", len(p.stops), len(p.minutes))
	}
	first, last := p.stops[0], p.stops[21]
	if first.id != "1000329" || first.name != "Autogara STP" || !first.placed || last.name != "Ghirbom T" || p.minutes[21] != 49 {
		t.Fatalf("ends %+v %+v, last minute %d", first, last, p.minutes[21])
	}
	for _, s := range p.stops {
		if s.placed == (s.name == "Straja T") {
			t.Fatalf("%s placed %v", s.name, s.placed)
		}
	}
	want := []departure{
		{day: calendar.Weekday, minute: 7*60 + 40}, {day: calendar.Saturday, minute: 7*60 + 40}, {day: calendar.Sunday, minute: 7*60 + 40},
		{day: calendar.Weekday, minute: 11*60 + 15, only: limit(time.Tuesday.String())},
		{day: calendar.Saturday, minute: 19*60 + 30}, {day: calendar.Sunday, minute: 19*60 + 30},
		{day: calendar.Weekday, minute: 23*60 + 30},
	}
	if !slices.Equal(p.departures, want) {
		t.Fatalf("departures %+v", p.departures)
	}
}

func TestLegend(t *testing.T) {
	for text, want := range map[string]limit{
		"Circulă doar în perioada școlii.":             term,
		"Circulă doar în perioada vacanțelor școlare.": holidays,
		"Circulă doar în zilele de marți.":             limit(time.Tuesday.String()),
	} {
		if got, ok := limitOf(text); !ok || got != want {
			t.Errorf("%q: %q", text, got)
		}
	}
	if _, ok := limitOf("Circulă la cerere."); ok {
		t.Error("an unknown legend has a meaning")
	}
}
