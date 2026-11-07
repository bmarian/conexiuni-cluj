package albaiulia

import (
	"slices"
	"strings"
	"testing"
)

func route(code, label string, stations ...string) page {
	p := page{code: code, label: label}
	for _, name := range stations {
		p.stops = append(p.stops, stop{id: name, name: name})
	}
	return p
}

func TestLines(t *testing.T) {
	pages := []page{
		route("101R_L1R", "101R", "P.ta Natiunii(L103)", "Unirea(L103)", "Gara CFR"),
		route("101T_L1T", "101T", "Gara", "Unirea(L104)", "P.ta Natiunii(L104)"),
		route("110_M1", "110", "Gara (Penny)", "Ampoi 3", "Micesti-gradinita", "MICESTI-Labului"),
		route("110B_Z1", "110B", "Gara (Penny)", "Ampoi 3", "Orizont", "Micesti-Zavoi"),
		route("110R_M2", "110R", "MICESTI-Labului", "Micesti-gradinita", "Ampoi 3", "Gara CFR"),
		route("110R_Z2", "110R", "Micesti-Zavoi", "Orizont", "Ampoi 3", "Gara CFR"),
		route("121_Am1", "121", "Ambient R", "Gara CFR"),
		route("121_Gm1", "121", "Gemina R", "Gara CFR"),
		route("440_IglTe", "440", "Ighiel", "Telna"),
		route("440_Telgl", "440", "Telna", "Ighiel"),
	}
	var got []string
	for _, l := range lines(pages) {
		ways := l.ways[0].code
		if l.ways[1] != nil {
			ways += "+" + l.ways[1].code
		}
		got = append(got, l.name+" "+ways)
	}
	want := []string{"101 101T_L1T+101R_L1R", "110 110_M1+110R_M2", "110B 110B_Z1+110R_Z2", "121 121_Am1", "121 121_Gm1", "440 440_IglTe+440_Telgl"}
	if !slices.Equal(got, want) {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

func TestStationName(t *testing.T) {
	for name, want := range map[string]string{
		"Seliste T": "Seliste", "Benic 1R": "Benic 1", "Oiejdea 1 R": "Oiejdea 1", "Recea-biser.R": "Recea-biser.",
		"Unirea(L103)": "Unirea", "Ardealul(l104)": "Ardealul", "LIDL(101)": "LIDL", "Galda de Jos(ram) R": "Galda de Jos(ram)",
		"Gara CFR": "Gara CFR", "BARABANT": "BARABANT", "VCST": "VCST", "Ampoi 3BT": "Ampoi 3BT",
	} {
		if got := stationName(name); got != want {
			t.Errorf("%q: %q", name, got)
		}
	}
}
