package transit

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"path/filepath"
	"slices"
	"testing"
)

func openHiddenStore(t *testing.T) *database.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func TestPassengerKeepsTheTimesOfTheStopsLeft(t *testing.T) {
	d := New(openHiddenStore(t))
	if err := d.UseHiddenStops([]int{1, 2, 4}); err != nil {
		t.Fatal(err)
	}
	row := func(trip string, stop int, offset float64) models.StopTime {
		return models.StopTime{TripID: trip, StopID: stop, OffsetArrivalTime: offset}
	}
	got := d.Passenger([]models.StopTime{
		row("a", 1, 90), row("a", 2, 30), row("a", 3, 40), row("a", 5, 10), row("a", 4, 50),
		row("b", 6, 0), row("b", 1, 20), row("b", 7, 15),
	})
	want := []models.StopTime{row("a", 3, 160), row("a", 5, 10), row("b", 6, 0), row("b", 7, 35)}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPassengerLeavesACityWithoutHiddenStopsAlone(t *testing.T) {
	rows := []models.StopTime{{TripID: "a", StopID: 1, OffsetArrivalTime: 9}}
	if got := New(openHiddenStore(t)).Passenger(rows); &got[0] != &rows[0] {
		t.Fatalf("rows were copied: %+v", got)
	}
}

func TestSeedNeverUndoesAnAdminsChoice(t *testing.T) {
	store := openHiddenStore(t)
	d := New(store)
	if err := d.UseHiddenStops([]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStopsHidden([]int{1}, false); err != nil {
		t.Fatal(err)
	}
	restarted := New(store)
	if err := restarted.UseHiddenStops([]int{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if got := restarted.HiddenStops(); !slices.Equal(got, []int{2, 3}) {
		t.Fatalf("hidden %v, want [2 3]", got)
	}
}
