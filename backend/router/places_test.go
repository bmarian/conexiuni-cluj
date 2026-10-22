package router

import "testing"

func tags(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestPlaceOf(t *testing.T) {
	if p, ok := placeOf(tags(map[string]string{"addr:street": "Strada Horea", "addr:housenumber": "3", "name": "Casa Horea"}), true); !ok || p.kind != placeAddress || p.number != "3" {
		t.Fatalf("address: %+v %v", p, ok)
	}
	if p, ok := placeOf(tags(map[string]string{"highway": "residential", "name": "Strada Horea"}), true); !ok || p.kind != placeStreet {
		t.Fatalf("street: %+v %v", p, ok)
	}
	if _, ok := placeOf(tags(map[string]string{"highway": "bus_stop", "name": "Piața Cipariu"}), false); ok {
		t.Fatal("a bus stop node is not a street")
	}
	if p, ok := placeOf(tags(map[string]string{"amenity": "bar", "name": "The Office Wine Bar"}), false); !ok || p.kind != placeNamed {
		t.Fatalf("named: %+v %v", p, ok)
	}
	if _, ok := placeOf(tags(map[string]string{"amenity": "bench"}), false); ok {
		t.Fatal("a nameless bench is nothing to find")
	}
	if p, ok := placeOf(tags(map[string]string{"place": "suburb", "name": "Mărăști"}), false); !ok || p.kind != placeDistrict {
		t.Fatalf("district: %+v %v", p, ok)
	}
}

func testPlaces() *Places {
	raw := []pbfPlace{
		{placeTags: placeTags{kind: placeTown, name: "Cluj-Napoca", big: true}, lat: 46.77, lon: 23.59, placed: true},
		{placeTags: placeTags{kind: placeTown, name: "Florești"}, lat: 46.745, lon: 23.49, placed: true},
		{placeTags: placeTags{kind: placeTown, name: "Baciu"}, lat: 46.80, lon: 23.52, placed: true},
		{placeTags: placeTags{kind: placeDistrict, name: "Mărăști"}, lat: 46.78, lon: 23.62, placed: true},
		{placeTags: placeTags{kind: placeDistrict, name: "Mănăștur"}, lat: 46.76, lon: 23.53, placed: true},
		{placeTags: placeTags{kind: placeStreet, name: "Strada Horea"}, lat: 46.772, lon: 23.585, placed: true},
		{placeTags: placeTags{kind: placeStreet, name: "Strada Horea"}, lat: 46.7721, lon: 23.5851, placed: true},
		{placeTags: placeTags{kind: placeStreet, name: "Strada Eroilor"}, lat: 46.746, lon: 23.491, placed: true},
		{placeTags: placeTags{kind: placeAddress, street: "Strada Horea", number: "3"}, lat: 46.7722, lon: 23.5852, placed: true},
		{placeTags: placeTags{kind: placeAddress, street: "Strada Horea", number: "3"}, lat: 46.7722, lon: 23.5853, placed: true},
		{placeTags: placeTags{kind: placeAddress, street: "Strada Horea", number: "31"}, lat: 46.7723, lon: 23.5854, placed: true},
		{placeTags: placeTags{kind: placeAddress, name: "Iulius Mall", street: "Strada Alexandru Vaida Voevod", number: "53B"}, lat: 46.774, lon: 23.628, placed: true},
		{placeTags: placeTags{kind: placeAddress, street: "Strada Constantin Brâncuși", number: "121"}, lat: 46.762, lon: 23.611, placed: true},
		{placeTags: placeTags{kind: placeNamed, name: "The Office Wine Bar", street: "Bulevardul 21 Decembrie 1989"}, lat: 46.779, lon: 23.612, placed: true},
		{placeTags: placeTags{kind: placeNamed, name: "ZTour", suburb: "Mărăști"}, lat: 46.781, lon: 23.621, placed: true},
	}
	return newPlaces(raw, BBox{46.70, 23.45, 46.84, 23.73})
}

func labels(found []Place) []string {
	out := make([]string, len(found))
	for i, p := range found {
		out[i] = p.Label
	}
	return out
}

func TestPlacesAreLabelledByTheirArea(t *testing.T) {
	p := testPlaces()
	if got := labels(p.Search("iulius", 1)); len(got) != 1 || got[0] != "Iulius Mall, Strada Alexandru Vaida Voevod, 53B, Mărăști" {
		t.Fatalf("got %q", got)
	}
	if got := labels(p.Search("eroilor", 1)); len(got) != 1 || got[0] != "Strada Eroilor, Florești" {
		t.Fatalf("got %q", got)
	}
}

func TestTheSameStreetOrAddressIsOnePlace(t *testing.T) {
	p := testPlaces()
	if got := labels(p.Search("strada horea", 10)); len(got) != 3 || got[0] != "Strada Horea, Cluj-Napoca" {
		t.Fatalf("got %q", got)
	}
}

func TestTheHouseNumberAskedForComesFirst(t *testing.T) {
	p := testPlaces()
	if got := labels(p.Search("horea 3", 10)); len(got) != 2 || got[0] != "Strada Horea, 3, Cluj-Napoca" || got[1] != "Strada Horea, 31, Cluj-Napoca" {
		t.Fatalf("got %q", got)
	}
}

func TestAccentsDoNotMatter(t *testing.T) {
	p := testPlaces()
	if got := labels(p.Search("brancusi 121", 1)); len(got) != 1 || got[0] != "Strada Constantin Brâncuși, 121, Cluj-Napoca" {
		t.Fatalf("got %q", got)
	}
	if got := labels(p.Search("MĂRĂȘTI", 1)); len(got) != 1 || got[0] != "Mărăști, Cluj-Napoca" {
		t.Fatalf("got %q", got)
	}
}

func TestADistrictBelongsToACityNotTheNearestVillage(t *testing.T) {
	p := testPlaces()
	if got := labels(p.Search("manastur", 1)); len(got) != 1 || got[0] != "Mănăștur, Cluj-Napoca" {
		t.Fatalf("got %q", got)
	}
}

func TestAPlaceNamedLikeTheQueryComesBeforeThoseInIt(t *testing.T) {
	p := testPlaces()
	if got := labels(p.Search("marasti", 2)); len(got) != 2 || got[0] != "Mărăști, Cluj-Napoca" || got[1] != "ZTour, Mărăști" {
		t.Fatalf("got %q", got)
	}
}

func TestEveryWordMustMatch(t *testing.T) {
	p := testPlaces()
	if got := p.Search("office wine", 5); len(got) != 1 || got[0].Label != "The Office Wine Bar, Bulevardul 21 Decembrie 1989, Mărăști" {
		t.Fatalf("got %+v", got)
	}
	if got := p.Search("office beer", 5); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestNearbyNamesAnAddressThenAStreet(t *testing.T) {
	p := testPlaces()
	if got, ok := p.Nearby(46.7722, 23.5852); !ok || got.Label != "Strada Horea, 3, Cluj-Napoca" {
		t.Fatalf("got %+v %v", got, ok)
	}
	if got, ok := p.Nearby(46.746, 23.4912); !ok || got.Label != "Strada Eroilor, Florești" {
		t.Fatalf("got %+v %v", got, ok)
	}
	if got, ok := p.Nearby(46.70, 23.45); ok {
		t.Fatalf("nothing is near the corner of the area, got %+v", got)
	}
}

func TestAreasAreTheTownsAndDistricts(t *testing.T) {
	raw := []pbfPlace{
		{placeTags: placeTags{kind: placeTown, name: "Cluj-Napoca", big: true, place: "city"}, lat: 46.77, lon: 23.59, placed: true},
		{placeTags: placeTags{kind: placeTown, name: "Florești", place: "village"}, lat: 46.745, lon: 23.49, placed: true},
		{placeTags: placeTags{kind: placeDistrict, name: "Mănăștur", place: "suburb"}, lat: 46.755, lon: 23.557, placed: true},
		{placeTags: placeTags{kind: placeDistrict, name: "Mănăștur", place: "suburb"}, lat: 46.756, lon: 23.558, placed: true},
		{placeTags: placeTags{kind: placeDistrict, name: "Gruia"}, lat: 46.777, lon: 23.577, placed: true},
	}
	got := newPlaces(raw, BBox{46.70, 23.45, 46.84, 23.73}).Areas()
	want := []Area{
		{Name: "Cluj-Napoca", Lat: 46.77, Lon: 23.59, Place: "city"},
		{Name: "Florești", Lat: 46.745, Lon: 23.49, Place: "village"},
		{Name: "Mănăștur", Lat: 46.755, Lon: 23.557, Place: "suburb"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("area %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestOnlyANodePlacesAnArea(t *testing.T) {
	if p, _ := placeOf(tags(map[string]string{"place": "suburb", "name": "Mărăști"}), true); p.place != "" {
		t.Fatalf("a way is placed by its first node: %+v", p)
	}
	if p, _ := placeOf(tags(map[string]string{"place": "village", "name": "Florești"}), false); p.place != "village" {
		t.Fatalf("node: %+v", p)
	}
}
