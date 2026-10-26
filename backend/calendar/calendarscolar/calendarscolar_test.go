package calendarscolar

import (
	"conexiuni-cluj/calendar"
	"strings"
	"testing"
)

func vevent(start, end, summary, description string) string {
	return "BEGIN:VEVENT\r\nUID:" + start + "@test\r\nDTSTART;VALUE=DATE:" + start + "\r\nDTEND;VALUE=DATE:" + end +
		"\r\nSUMMARY:" + summary + "\r\nDESCRIPTION:" + description + "\r\nEND:VEVENT\r\n"
}

var clujFeed = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-CALNAME:Calendar Școlar - Cluj\r\n" +
	vevent("20260411", "20260422", "Vacanța de primăvară 2026", `Vacanța de Paște (11-21 aprilie 2026\, include Paștele Ortodox 12 aprilie)`) +
	vevent("20260619", "20260620", "Sfârșit an școlar 2025-2026", "Ultima zi de școală 2025-2026") +
	vevent("20260620", "20260907", "Vacanța de vară 2026", "Vacanța mare de vară 2026") +
	vevent("20260907", "20260908", "Început an școlar 2026-2027", "Prima zi de școală") +
	vevent("20261005", "20261006", "5 Octombrie - Ziua internațională a educației", "Ziua internațională a educației - nu se organizează\r\n  cursuri (conform Art. 3)") +
	vevent("20261024", "20261102", "Vacanța de toamnă", "Vacanța de toamnă (24 octombrie - 1 noiembrie 2026)") +
	vevent("20261130", "20261201", "30 Noiembrie - Sfântul Andrei", "Sărbătoare legală - zi liberă") +
	vevent("20261201", "20261202", "1 Decembrie - Ziua Națională", "Ziua Națională a României - zi liberă") +
	vevent("20261223", "20270111", "Vacanța de iarnă", "Vacanța de Crăciun și Anul Nou") +
	vevent("20270111", "20270112", "Început semestru II", "Reluarea cursurilor") +
	vevent("20270124", "20270125", "24 Ianuarie - Unirea Principatelor", "Ziua Unirii Principatelor Române - zi liberă (duminică)") +
	vevent("20270215", "20270222", "Vacanța intersemestrială", "Vacanța intersemestrială pentru Cluj (Grupa A)") +
	vevent("20270424", "20270505", "Vacanța de primăvară", "Vacanța de Paște") +
	vevent("20270501", "20270502", "1 Mai - Ziua Muncii", "Ziua Internațională a Muncii - zi liberă (sâmbătă)") +
	vevent("20270601", "20270602", "1 Iunie - Ziua Copilului", "Ziua Copilului - zi liberă") +
	vevent("20270611", "20270612", "Ultima zi - clasa a VIII-a", "Ultima zi de cursuri pentru elevii clasei a VIII-a") +
	vevent("20270618", "20270619", "Sfârșit an școlar", "Ultima zi de școală pentru clasele I-VII și IX-XI") +
	vevent("20270619", "20270906", "Vacanța de vară", "Vacanța mare de vară (19 iunie - 5 septembrie 2027)") +
	vevent("20270620", "20270622", "Rusalii", "Rusalii (20-21 iunie 2027)") +
	"END:VCALENDAR\r\n"

func TestParse(t *testing.T) {
	years := Parse([]byte(clujFeed))
	want := calendar.SchoolYear{Year: 2026, Start: "2026-09-07", End: "2027-06-18", Breaks: []calendar.Range{
		{Name: "Ziua internațională a educației", Start: "2026-10-05", End: "2026-10-05"},
		{Name: "Vacanța de toamnă", Start: "2026-10-24", End: "2026-11-01"},
		{Name: "Vacanța de iarnă", Start: "2026-12-23", End: "2027-01-10"},
		{Name: "Vacanța intersemestrială", Start: "2027-02-15", End: "2027-02-21"},
		{Name: "Vacanța de primăvară", Start: "2027-04-24", End: "2027-05-04"},
	}}
	if len(years) != 1 || !years[0].Equal(want) {
		t.Fatalf("got %+v\nwant %+v", years, want)
	}
	if err := years[0].Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestParseNeedsBothSummers(t *testing.T) {
	half := clujFeed[:strings.Index(clujFeed, "BEGIN:VEVENT\r\nUID:20270619@")] + "END:VCALENDAR\r\n"
	if years := Parse([]byte(half)); len(years) != 0 {
		t.Fatalf("a year without its end: %+v", years)
	}
}

func TestBreakName(t *testing.T) {
	for in, want := range map[string]string{
		"Vacanța de toamnă":                             "Vacanța de toamnă",
		"5 Octombrie - Ziua internațională a educației": "Ziua internațională a educației",
		strings.Repeat("ă", 30):                         strings.Repeat("ă", 20),
	} {
		if got := breakName(in); got != want {
			t.Errorf("breakName(%q) = %q, want %q", in, got, want)
		}
	}
}
