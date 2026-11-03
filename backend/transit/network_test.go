package transit

import (
	"conexiuni-cluj/feeds"
	"reflect"
	"testing"
)

func TestANewNetworkReplacesTheRuns(t *testing.T) {
	store := openHiddenStore(t)
	d := New(store)
	if err := d.StoreNetwork(&feeds.Snapshot{Runs: map[string]int{"a": 0, "b": 1}}); err != nil {
		t.Fatal(err)
	}
	got, err := store.RunDirections([]string{"a", "b", "unknown"})
	if err != nil || !reflect.DeepEqual(got, map[string]int{"a": 0, "b": 1}) {
		t.Fatalf("runs %v, %v", got, err)
	}
	if err := d.StoreNetwork(&feeds.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.RunDirections([]string{"a"}); err != nil || len(got) != 0 {
		t.Fatalf("runs left over: %v, %v", got, err)
	}
}
