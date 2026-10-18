package handlers

import (
	"conexiuni-cluj/database"
	"path/filepath"
	"testing"
)

func newTestCity(t *testing.T) *City {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"), filepath.Join(dir, "archive.db"), 5)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	city := NewCity("test", "Test", store, nil, nil, nil, nil, Options{})
	city.resetProfileSnapshots()
	t.Cleanup(city.Close)
	return city
}

func (city *City) withoutArchive(fn func()) {
	archive := city.Archive
	city.Archive = nil
	defer func() { city.Archive = archive }()
	fn()
}
