package tranzy

import (
	"conexiuni-cluj/models"
	"testing"
)

func TestLineColor(t *testing.T) {
	cases := []struct {
		name string
		kind models.RouteType
		want string
	}{
		{"25N", models.Bus, "#2D2E83"},
		{"M26N", models.Bus, "#2D2E83"},
		{"5N", models.Trolleybus, "#2D2E83"},
		{"N104", models.Bus, "#2D2E83"},
		{"101", models.Tram, "#BE1622"},
		{"5", models.Trolleybus, "#008D36"},
		{"8L", models.Bus, "#1D71B8"},
		{"RoHu", models.Bus, "#1D71B8"},
		{"13TV", models.Bus, "#1D71B8"},
	}
	for _, c := range cases {
		if got := lineColor(models.Route{RouteShortName: c.name, RouteType: c.kind}); got != c.want {
			t.Errorf("lineColor(%s) = %s, want %s", c.name, got, c.want)
		}
	}
}
