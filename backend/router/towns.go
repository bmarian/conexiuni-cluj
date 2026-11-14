package router

import "io"

type Town struct {
	Name     string
	Lat, Lon float64
}

func cityOrTown(tag func(string) string, way bool) (placeTags, bool) {
	switch tag("place") {
	case "city", "town":
		name := tag("name")
		return placeTags{kind: placeTown, name: name}, !way && name != ""
	}
	return placeTags{}, false
}

func Towns(r io.ReadSeeker) ([]Town, error) {
	defer holdExtract()()
	var out []Town
	err := read(r, pbfFilter{
		node:  func(int64, float64, float64) (bool, bool) { return false, true },
		way:   func(int64, func(string) string) (string, bool) { return "", false },
		place: cityOrTown,
	}, func(b pbfBlock) {
		for _, p := range b.places {
			out = append(out, Town{Name: p.name, Lat: p.lat, Lon: p.lon})
		}
	})
	return out, err
}
