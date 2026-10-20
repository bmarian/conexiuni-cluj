package live

import (
	"testing"
)

func TestDayOfType(t *testing.T) {
	eng := newTestCity(t)
	wednesday := weekdayAt(23, 10, 0)
	for dayType, want := range map[string]int{"weekday": 23, "saturday": 26, "sunday": 27, "bogus": 23} {
		if got := eng.DayOfType(wednesday, dayType).Day(); got != want {
			t.Errorf("%s: day %d, want %d", dayType, got, want)
		}
	}
}
