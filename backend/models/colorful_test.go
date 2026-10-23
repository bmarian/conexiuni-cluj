package models

import "testing"

func TestColorfulRouteColor(t *testing.T) {
	for name, want := range map[string]string{
		"25": "#462EE0", "35": "#DA771B", "24B": "#A119D7", "M26N": "#32C322", "8L": "#2A52CB",
		"101": "#DD1DBA", " 5n ": "#2552E4", "Ș1": "#1FC16B", "RoHu": "#DE3581", "N104": "#70C328", "": FallbackRouteColor,
	} {
		if got := ColorfulRouteColor(name); got != want {
			t.Errorf("ColorfulRouteColor(%q) = %s, want %s", name, got, want)
		}
	}
}
