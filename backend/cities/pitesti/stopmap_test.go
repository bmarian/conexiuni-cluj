package pitesti

import (
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"testing"
)

func TestStopMap(t *testing.T) {
	page := `<span class="txt_slogan">Harta statiilor Publitrans 2000 S.A.</span><div class="tablefull">` +
		`<iframe src="https://www.google.com/maps/d/u/1/embed?mid=1LFjw7gUHG6n-19eYsTNX667Iz1hI4KI&ehbc=2E312F&noprof=1" width="640" height="480"></iframe></div>`
	got, err := stopMapURL([]byte(page))
	if want := "https://www.google.com/maps/d/kml?forcekml=1&mid=1LFjw7gUHG6n-19eYsTNX667Iz1hI4KI"; err != nil || got != want {
		t.Fatalf("got %q %v", got, err)
	}
	kml := `<?xml version="1.0" encoding="UTF-8"?><kml xmlns="http://www.opengis.net/kml/2.2"><Document><name>Toate statiile</name><Folder><name>Toate statiile</name>` +
		`<Placemark><name>Hotel Ramada 2</name><styleUrl>#icon-1532-673AB7-labelson</styleUrl><Point><coordinates>
            24.87651,44.865208,0
          </coordinates></Point></Placemark>` +
		`<Placemark><name>Gavana - Razboieni</name><LineString><coordinates>24.84,44.87,0 24.85,44.86,0</coordinates></LineString></Placemark>` +
		`<Placemark><name>Cap Linie Razboieni</name><Point><coordinates>24.876562,44.841058,0</coordinates></Point></Placemark></Folder></Document></kml>`
	markers, err := parseStopMap([]byte(kml))
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 2 || markers[0] != (marker{"Hotel Ramada 2", router.Point{Lat: 44.865208, Lon: 24.87651}}) || markers[1].name != "Cap Linie Razboieni" {
		t.Fatalf("markers %+v", markers)
	}
}

func onMap(names ...string) way {
	var w way
	for _, name := range names {
		w.stops = append(w.stops, sitegtfs.Stop{ID: name, Name: name})
	}
	return w
}

func TestPlace(t *testing.T) {
	markers := []marker{
		{"Auchan", router.Point{Lat: 44.82347, Lon: 24.9097}},
		{"Auchan", router.Point{Lat: 44.88446, Lon: 24.82832}},
		{"Cap Linie B.A.T. Bascov", router.Point{Lat: 44.88894, Lon: 24.82316}},
		{"Cap Linie B.A.T. Bascov 1", router.Point{Lat: 44.88901, Lon: 24.82342}},
		{"Cap Linie Razboieni", router.Point{Lat: 44.84106, Lon: 24.87656}},
		{"Cap Linie Trivale", router.Point{Lat: 44.84864, Lon: 24.84324}},
		{"Cap Linie Trivale 2", router.Point{Lat: 44.84889, Lon: 24.84366}},
		{"Trivale Complex Ii 1", router.Point{Lat: 44.84971, Lon: 24.84768}},
		{"Spitalul Militar", router.Point{Lat: 44.86507, Lon: 24.86196}},
		{"Spitalul Militar 1", router.Point{Lat: 44.86437, Lon: 24.86248}},
		{"Hotel Ramada", router.Point{Lat: 44.8644, Lon: 24.87712}},
	}
	pages := []linePage{
		{ways: []way{onMap("Auchan", "B.A.T. Bascov 1"), onMap("Cap de Linie Razboieni", "Auchan")}},
		{ways: []way{onMap("Cap Linie Trivale 1", "Trivale Complex Ii 1", "Spitalul Militar 2", "Hotel Ramada 1", "Hotel Ramada", "Stoienari 1")}},
	}
	place(pages, markers)
	for _, c := range []struct {
		stop     sitegtfs.Stop
		id, name string
	}{
		{pages[0].ways[0].stops[0], "Auchan#2", "Auchan"},
		{pages[0].ways[0].stops[1], "Cap Linie B.A.T. Bascov 1", "B.A.T. Bascov"},
		{pages[0].ways[1].stops[0], "Cap Linie Razboieni", "Cap de Linie Razboieni"},
		{pages[0].ways[1].stops[1], "Auchan", "Auchan"},
		{pages[1].ways[0].stops[0], "Cap Linie Trivale", "Cap Linie Trivale"},
		{pages[1].ways[0].stops[2], "Spitalul Militar", "Spitalul Militar"},
		{pages[1].ways[0].stops[4], "Hotel Ramada", "Hotel Ramada"},
	} {
		if !c.stop.Placed || c.stop.ID != c.id || c.stop.Name != c.name {
			t.Errorf("%+v, want %s %q", c.stop, c.id, c.name)
		}
	}
	// A page names the unnumbered "Hotel Ramada" itself, so it is not taken for side 1.
	for _, s := range []sitegtfs.Stop{pages[1].ways[0].stops[3], pages[1].ways[0].stops[5]} {
		if s.Placed {
			t.Errorf("%s placed at %v", s.ID, s.At)
		}
	}
}

func TestStationName(t *testing.T) {
	for name, want := range map[string]string{
		"Lanariei 1": "Lanariei", "Spitalul Militar 2": "Spitalul Militar", "Scoala Nr. 2-1": "Scoala Nr. 2", "C.A.P. 1": "C.A.P.",
		"Cap Linie Scoala Primara nr.1": "Cap Linie Scoala Primara nr.1", "Scoala Generala Nr. 8": "Scoala Generala Nr. 8",
		"Argeselu 1 Dn73": "Argeselu 1 Dn73", "Cap Linie Trivale I (Biserica)": "Cap Linie Trivale I (Biserica)",
	} {
		if got := stationName(name); got != want {
			t.Errorf("%q: %q", name, got)
		}
	}
}
