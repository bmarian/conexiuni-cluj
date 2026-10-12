package handlers

import (
	"testing"
)

func TestDayOfType(t *testing.T) {
	wednesday := weekdayAt(23, 10, 0)
	for dayType, want := range map[string]int{"weekday": 23, "saturday": 26, "sunday": 27, "bogus": 23} {
		if got := dayOfType(wednesday, dayType).Day(); got != want {
			t.Errorf("%s: day %d, want %d", dayType, got, want)
		}
	}
}
