package ramnicuvalcea

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/sitegtfs"
	"conexiuni-cluj/router"
	"maps"
	"os"
	"slices"
	"testing"
)

func TestIndex(t *testing.T) {
	index := `<h3>Autobuze</h3><div class="row trasee"><div><a title="Traseul 5A" class="number mr-3 float-left" data-route_id="51" href="/t/5A">` +
		`<div class="card traseu mb-4"><img class="card-img" src="/images/trasee/image-t5A.jpg" alt="Traseul 5A" /><div class="card-img-overlay p-0"><h2 class="card-title">5A</h2><p class="card-footer itinerar mb-0">Dispecerat Nord - Metnef(1 Mai)</p></div></div></a></div></div>` +
		`<h3>Microbuze</h3><div class="row trasee" data-extra="microbuz"><div><a title="Traseul B" class="number mr-3 float-left" data-route_id="21" href="/t/B">` +
		`<div class="card traseu mb-4"><div class="card-img-overlay p-0"><h2 class="card-title">B</h2><p class="card-footer itinerar mb-0">Dispecerat Nord - Buda</p></div></div></a></div></div>` +
		`<a href="/statii">Stații</a>`
	links, err := parseIndex([]byte(index), "https://eta-bus.ro/trasee")
	if err != nil {
		t.Fatal(err)
	}
	want := []link{{url: "https://eta-bus.ro/t/5A", name: "5A"}, {url: "https://eta-bus.ro/t/B", name: "B"}}
	if !slices.Equal(links, want) {
		t.Fatalf("got %+v", links)
	}
	if got := parseUpdated([]byte(`<p>Ultima actualizare a programului: 01 Sep 2026</p>`)); got != "01 Sep 2026" {
		t.Errorf("updated %q", got)
	}
}

func TestMap(t *testing.T) {
	page := `var link_statie = document.createElement('a');
      link_statie.href = '/s/1';
      link_statie.textContent = 'Programul statiei';
      infowincontent1.appendChild(link_statie);
      var marker1 = new google.maps.Marker({position: {lat: 45.1059298583, lng: 24.3294807181}, title: 'Arinilor Tur', icon: 'https://maps.gstatic.com/mapfiles/ms2/micons/red-dot.png'});
      marker1.setMap(map);
      marker1.addListener('click', function () {
        infoWindow.open(map, marker1);
      });
      link_statie.href = '/s/47';
      infowincontent47.appendChild(link_statie);
      var marker47 = new google.maps.Marker({position: {lat: 45.0597, lng: 24.3511}, title: 'Cap Linie Colonie Nuci', icon: 'https://maps.gstatic.com/mapfiles/ms2/micons/red-dot.png'});`
	want := map[string]router.Point{"1": {Lat: 45.1059298583, Lon: 24.3294807181}, "47": {Lat: 45.0597, Lon: 24.3511}}
	if got := parseMap([]byte(page)); !maps.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestLine(t *testing.T) {
	data, err := os.ReadFile("testdata/line13.html")
	if err != nil {
		t.Fatal(err)
	}
	w, err := parseLine(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(w[0]) != 13 || len(w[1]) != 15 {
		t.Fatalf("%d and %d stops", len(w[0]), len(w[1]))
	}
	ends := []sitegtfs.Stop{w[0][0], w[0][12], w[1][0], w[1][14]}
	want := []sitegtfs.Stop{{ID: "73", Name: "Dispecerat Nord"}, {ID: "188", Name: "Clinica Sofianu Retur"}, {ID: "60", Name: "Clinica Sofianu Tur"}, {ID: "73", Name: "Dispecerat Nord"}}
	if !slices.Equal(ends, want) {
		t.Fatalf("ends %+v", ends)
	}
}

func TestStop(t *testing.T) {
	data, err := os.ReadFile("testdata/stop261.html")
	if err != nil {
		t.Fatal(err)
	}
	passings, err := parseStop(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(passings) != 16 {
		t.Fatalf("%d runs", len(passings))
	}
	weekend := []string{calendar.Saturday, calendar.Sunday}
	for i, want := range map[int]passing{
		0:  {stop: "261", line: "9", way: 0, days: []string{calendar.Weekday}, trip: "11_00008_0_0", minute: 7*60 + 5},
		2:  {stop: "261", line: "9", way: 0, days: []string{calendar.Weekday}, trip: "9_00093_0_0", minute: 14*60 + 50, only: sitegtfs.Term},
		5:  {stop: "261", line: "9", way: 1, days: []string{calendar.Weekday}, trip: "11_00010_0_1", minute: 5*60 + 30},
		15: {stop: "261", line: "9", way: 1, days: weekend, trip: "11_00018_0_1", minute: 16*60 + 45},
	} {
		got := passings[i]
		if got.stop != want.stop || got.line != want.line || got.way != want.way || got.trip != want.trip || got.minute != want.minute || got.only != want.only || !slices.Equal(got.days, want.days) {
			t.Errorf("run %d: %+v, want %+v", i, got, want)
		}
	}
}
