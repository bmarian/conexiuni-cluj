package satumare

import (
	"conexiuni-cluj/calendar"
	"os"
	"slices"
	"testing"
)

func TestIndex(t *testing.T) {
	index := `<a href="https://www.tusm.ro/liniistatiiorare?linia=1">1</a><a href="https://www.tusm.ro/liniistatiiorare?linia=15 SD">15 SD</a>` +
		`<a href="https://www.tusm.ro/liniistatiiorare?linia=1">1</a><a href="https://www.tusm.ro/servicii-si-tarife">Tarife</a>`
	links, err := parseIndex([]byte(index), "https://www.tusm.ro/liniistatiiorare")
	if err != nil {
		t.Fatal(err)
	}
	want := []link{
		{line: "1", url: "https://www.tusm.ro/liniistatiiorare?linia=1"},
		{line: "15 SD", url: "https://www.tusm.ro/liniistatiiorare?linia=15+SD"},
	}
	if !slices.Equal(links, want) {
		t.Fatalf("got %+v", links)
	}
}

// Line 10's page as tusm.ro served it on 2026-11-07, without its scripts, images and station lists.
func line10(t *testing.T) []route {
	data, err := os.ReadFile("testdata/10.html")
	if err != nil {
		t.Fatal(err)
	}
	routes, err := parsePage(data)
	if err != nil {
		t.Fatal(err)
	}
	return routes
}

func TestPage(t *testing.T) {
	routes := line10(t)
	var labels []string
	for _, r := range routes {
		labels = append(labels, r.label)
	}
	if !slices.Equal(labels, []string{"10", "10A", "10B", "10C", "10D"}) {
		t.Fatalf("routes %v", labels)
	}
	main := routes[0]
	if main.from != [2]string{"Ambudului", "Auchan"} {
		t.Fatalf("from %q", main.from)
	}
	for _, c := range []struct {
		way         int
		kind        string
		count       int
		first, last string
	}{
		{0, calendar.Weekday, 14, "05:30", "20:30"},
		{0, calendar.Saturday, 15, "05:30", "20:30"},
		{0, calendar.Sunday, 16, "05:30", "20:30"},
		{1, calendar.Weekday, 13, "05:50", "21:15"},
		{1, calendar.Sunday, 16, "05:50", "21:15"},
	} {
		times := main.times[c.way][c.kind]
		if len(times) != c.count || times[0] != c.first || times[len(times)-1] != c.last {
			t.Fatalf("%d %s: %v", c.way, c.kind, times)
		}
	}
	if routes[3].runs() {
		t.Fatalf("10C has times %v", routes[3].times)
	}
	// "Sărbători și weekend" is both days, "Sâmbătă" and "Duminică" one each.
	d := routes[4].times
	if !slices.Equal(d[0][calendar.Saturday], []string{"21:30"}) || !slices.Equal(d[0][calendar.Sunday], []string{"21:30"}) ||
		!slices.Equal(d[1][calendar.Saturday], []string{"22:25"}) || !slices.Equal(d[1][calendar.Sunday], []string{"22:30"}) {
		t.Fatalf("10D %v", d)
	}
}
