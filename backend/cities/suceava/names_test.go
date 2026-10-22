package suceava

import "testing"

func TestNamesMatchWordForWord(t *testing.T) {
	for _, c := range []struct {
		station, stop string
		same          bool
	}{
		{"SPITAL", "Spitalul Judetean", true},
		{"STR. HUMORULUI", "Strada Humorului", true},
		{"ȘCOALA GENERALĂ NR. 9", "Scoala Generala Nr. 9 - Ion Creangă", true},
		{"CARPATIS", "Moara Carp", false},
		{"PECO CUMPĂRĂTURA", "C.a.p", false},
		{"ȘCOALA NR. 9", "Scoala Nr. 90", false},
		{"3 MOVILE", "Trei Movile", true},
		{"3 MOVILE", "Strada Movilei, 3, Șcheia", true},
	} {
		words := nameWords(c.station)
		if got := wordsShared(words, nameWords(c.stop)) == len(words); got != c.same {
			t.Errorf("%q in %q: %v, want %v", c.station, c.stop, got, c.same)
		}
	}
}

func TestNumbersAreAlsoSearchedSpelledOut(t *testing.T) {
	if got := spellings("3 MOVILE"); len(got) != 2 || got[1] != "trei MOVILE" {
		t.Fatalf("spellings %q", got)
	}
	if got := spellings("TINOASA 2A"); len(got) != 1 {
		t.Fatalf("spellings %q", got)
	}
}
