package transit

import (
	"testing"
)

func TestParseDepartureMinutesIgnoresAnnotations(t *testing.T) {
	for in, want := range map[string]int{"*07:25": 445, "07:25*": 445, "24:15*": 1455} {
		if got, ok := ParseDeparture(in); !ok || got != want {
			t.Errorf("ParseDeparture(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
}
