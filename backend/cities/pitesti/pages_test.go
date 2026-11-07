package pitesti

import (
	"conexiuni-cluj/calendar"
	"os"
	"slices"
	"testing"
)

func TestIndex(t *testing.T) {
	index := `<table><tr><th class="txt_orange" >Nr crt</th><th class="txt_orange" >Denimire</th></tr>` +
		`<tr><td style='background-color: #f7901e;'>1</td><td style='background-color: #f7901e;'>Linia M11</td><td>Pitesti - Schiau</td>` +
		`<td style='background-color: #ffffff; '><a href='1_2.php?id=11' style='color: #000000;' ><img src='images/plus_2.png'></a></td></tr>` +
		`<tr><td>2</td><td>Linia 2B</td><td>Trivale - Arpechim</td><td><a href='1_2.php?id=20'><img src='images/plus_2.png'></a></td></tr></table>` +
		`<a href="linii_oras.php">LINII URBANE</a>`
	links, err := parseIndex([]byte(index), "https://www.publitrans2000.ro/linii_metropolitane")
	if err != nil {
		t.Fatal(err)
	}
	want := []link{
		{url: "https://www.publitrans2000.ro/1_2.php?id=11", id: "11", name: "M11"},
		{url: "https://www.publitrans2000.ro/1_2.php?id=20", id: "20", name: "2B"},
	}
	if !slices.Equal(links, want) {
		t.Fatalf("got %+v", links)
	}
}

func readPage(t *testing.T, file string) linePage {
	t.Helper()
	data, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	p, err := parsePage(data, link{id: file})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func names(w way) []string {
	var out []string
	for _, s := range w.stops {
		out = append(out, s.Name)
	}
	return out
}

func TestPage(t *testing.T) {
	p := readPage(t, "c.html")
	if len(p.ways) != 2 {
		t.Fatalf("%d ways", len(p.ways))
	}
	out, back := p.ways[0], p.ways[1]
	if got := names(out); !slices.Equal(got, []string{"Teatrul Al. Davila", "Smeurei 2", "Cap Linie Cimitir Sf. Gheorghe"}) {
		t.Errorf("way out %v", got)
	}
	if got := names(back); !slices.Equal(got, []string{"Cap Linie Cimitir Sf. Gheorghe", "Gradina Publica"}) {
		t.Errorf("way back %v", got)
	}
	// The way out's end lists the way back's departures, so its runs stop short of it.
	if len(out.runs[calendar.Weekday]) != 0 || len(out.runs[calendar.Saturday]) != 6 || !slices.Equal(out.runs[calendar.Saturday][0], []int{8 * 60, 8*60 + 3}) {
		t.Errorf("way out runs %v", out.runs)
	}
	if got := back.runs[calendar.Sunday]; len(got) != 6 || !slices.Equal(got[5], []int{13*60 + 50, 13*60 + 54}) {
		t.Errorf("way back sundays %v", got)
	}
}

func TestAfterMidnight(t *testing.T) {
	p := readPage(t, "m10.html")
	out, back := p.ways[0], p.ways[1]
	if len(out.stops) != 3 || len(out.runs[calendar.Weekday]) != 19 || !slices.Equal(out.runs[calendar.Weekday][0], []int{4*60 + 40, 4*60 + 42}) {
		t.Errorf("way out %d stops, weekdays %v", len(out.stops), out.runs[calendar.Weekday])
	}
	weekdays := back.runs[calendar.Weekday]
	if len(weekdays) != 20 || !slices.Equal(weekdays[19], []int{24*60 + 2, 24*60 + 4}) {
		t.Errorf("way back weekdays %v", weekdays)
	}
}

func TestCircuit(t *testing.T) {
	p := readPage(t, "1.html")
	if len(p.ways) != 1 {
		t.Fatalf("%d ways", len(p.ways))
	}
	c := p.ways[0]
	if got := names(c); !slices.Equal(got, []string{"Cap Linie Tudor Vladimirescu", "Lanariei 1", "Lanariei 2"}) {
		t.Errorf("stops %v", got)
	}
	if got := c.runs[calendar.Weekday]; len(got) != 6 || !slices.Equal(got[0], []int{6*60 + 30, 6*60 + 32, 7*60 + 6}) {
		t.Errorf("weekdays %v", got)
	}
	if len(c.runs[calendar.Saturday]) != 0 {
		t.Errorf("saturdays %v", c.runs[calendar.Saturday])
	}
}
