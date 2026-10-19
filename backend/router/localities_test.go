package router

import (
	"slices"
	"testing"
)

func square(name string, level int, minLat, minLon, maxLat, maxLon float64) boundary {
	b := boundary{pbfRelation: pbfRelation{name: name, level: level}, box: BBox{minLat, minLon, maxLat, maxLon}}
	b.ring(minLat, minLon, maxLat, maxLon)
	return b
}

func (b *boundary) ring(minLat, minLon, maxLat, maxLon float64) {
	corners := []Point{{minLat, minLon}, {minLat, maxLon}, {maxLat, maxLon}, {maxLat, minLon}, {minLat, minLon}}
	for i := 1; i < len(corners); i++ {
		b.ends = append(b.ends, corners[i-1], corners[i])
	}
}

func town(name string, lat, lon float64) pbfPlace {
	return pbfPlace{placeTags: placeTags{kind: placeTown, name: name}, lat: lat, lon: lon, placed: true}
}

// Bucharest, a hole in its county, with Pantelimon's centre nearer its eastern edge than its own.
func testBounds() ([]boundary, []pbfPlace) {
	county := square("Ilfov", adminCounty, 43.9, 25.9, 44.3, 26.4)
	county.ring(44.0, 26.0, 44.2, 26.2)
	bounds := []boundary{
		county,
		square("Municipiul București", adminCounty, 44.0, 26.0, 44.2, 26.2),
		square("Pantelimon", adminTown, 44.0, 26.2, 44.2, 26.3),
		square("Comuna 1 Decembrie", adminTown, 43.9, 26.0, 44.0, 26.2),
	}
	places := []pbfPlace{
		town("București", 44.1, 26.1),
		town("Pantelimon", 44.1, 26.21),
		town("1 Decembrie", 43.95, 26.05),
		town("Copăceni", 43.95, 26.15),
	}
	return bounds, places
}

func TestACityEdgeIsTheCityNotTheNearerTown(t *testing.T) {
	bounds, places := testBounds()
	towns, _ := locate([]Point{{44.1, 26.19}, {44.1, 26.25}}, &segments{grid: map[[2]int32][]int32{}}, nil, places, bounds)
	if !slices.Equal(towns, []string{"București", "Pantelimon"}) {
		t.Fatalf("towns %q", towns)
	}
}

func TestAVillageIsTheNearestInsideItsCommune(t *testing.T) {
	bounds, places := testBounds()
	// Nearer Bucharest's centre than Copăceni's, but in the commune, next to Copăceni.
	towns, _ := locate([]Point{{43.99, 26.12}, {43.94, 26.02}}, &segments{grid: map[[2]int32][]int32{}}, nil, places, bounds)
	if !slices.Equal(towns, []string{"Copăceni", "1 Decembrie"}) {
		t.Fatalf("towns %q", towns)
	}
}

func TestOutsideEveryPlaceTheBoundaryNamesIt(t *testing.T) {
	bounds := []boundary{square("Comuna Vidra", adminTown, 44.0, 26.0, 44.1, 26.1)}
	towns, _ := locate([]Point{{44.05, 26.05}, {45, 27}}, &segments{grid: map[[2]int32][]int32{}}, nil, nil, bounds)
	if !slices.Equal(towns, []string{"Comuna Vidra", ""}) {
		t.Fatalf("towns %q", towns)
	}
}

func TestTheStreetIsTheNearestNamedRoad(t *testing.T) {
	roads := &segments{grid: map[[2]int32][]int32{}, box: BBox{44.0, 26.0, 44.01, 26.01}}
	roads.lat = []float64{44.0050, 44.0050, 44.0, 44.01}
	roads.lon = []float64{26.0, 26.01, 26.0054, 26.0054}
	roads.addSegment(0, 1)
	roads.addSegment(2, 3)
	// 11 m from the first road, 31 m from the second.
	_, streets := locate([]Point{{44.0051, 26.005}}, roads, []string{"Strada Gării", "Strada Argeș"}, nil, nil)
	if streets[0] != "Strada Gării" {
		t.Fatalf("street %q", streets[0])
	}
}
