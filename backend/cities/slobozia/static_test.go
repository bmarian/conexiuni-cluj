package slobozia

import (
	"os"
	"testing"
)

func TestBundleURL(t *testing.T) {
	page, err := os.ReadFile("testdata/home.html")
	if err != nil {
		t.Fatal(err)
	}
	got, err := bundleURL(page, "https://www.sptl.ro/")
	if err != nil || got != "https://www.sptl.ro/assets/busData-DuiQwkdr.js" {
		t.Fatalf("%q, %v", got, err)
	}
	if _, err := bundleURL([]byte(`<html><head><link rel="stylesheet" href="/assets/style-D4rTsWa-.css"></head></html>`), "https://www.sptl.ro/"); err == nil {
		t.Fatal("a page without the bundle names one")
	}
}
