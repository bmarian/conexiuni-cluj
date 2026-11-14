package search

import "testing"

func TestReadTakesOutHints(t *testing.T) {
	cases := []struct {
		query, text, show string
		only              only
		tickets           bool
	}{
		{"orar 25", "25", showTimetable, anyKind, false},
		{"Linia 26 live", "26", showMap, linesOnly, false},
		{"statia victoria", "victoria", "", stopsOnly, false},
		{"pret bilet", "pret bilet", "", anyKind, true},
		{"orar", "orar", "", anyKind, false},
		{"Grădinița cu orar prelungit", "gradinita cu prelungit", showTimetable, anyKind, false},
	}
	for _, c := range cases {
		r := read(c.query)
		if r.text != c.text || r.show != c.show || r.only != c.only || r.tickets != c.tickets {
			t.Errorf("read(%q) = %+v", c.query, r)
		}
	}
}

func TestTripEnds(t *testing.T) {
	cases := []struct{ query, from, to string }{
		{"de la gara la vivo", "gara", "vivo"},
		{"gara la vivo", "gara", "vivo"},
		{"Gara - VIVO", "gara", "vivo"},
		{"gara → vivo", "gara", "vivo"},
		{"din gara in centru", "gara", "centru"},
		{"spre vivo", "", "vivo"},
		{"la vivo", "", "vivo"},
		{"de la gara pana la vivo", "gara", "vivo"},
	}
	for _, c := range cases {
		from, to, ok := tripEnds(c.query)
		if !ok || from != c.from || to != c.to {
			t.Errorf("tripEnds(%q) = %q, %q, %v", c.query, from, to, ok)
		}
	}
	for _, q := range []string{"gara vivo", "piata unirii", "cluj-napoca", "statia in"} {
		if _, _, ok := tripEnds(q); ok {
			t.Errorf("tripEnds(%q) read a trip", q)
		}
	}
}
