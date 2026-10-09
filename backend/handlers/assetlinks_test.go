package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestAssetLinksNamesTheAppAndItsCertificate(t *testing.T) {
	app := fiber.New()
	app.Get("/.well-known/assetlinks.json", AssetLinksHandler("online.bmarian.bus", []string{"ab:cd"}))
	resp, err := app.Test(httptest.NewRequest("GET", "/.well-known/assetlinks.json", nil))
	if err != nil {
		t.Fatal(err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" && ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	var links []assetLink
	if err := json.Unmarshal(body, &links); err != nil || len(links) != 1 {
		t.Fatalf("links = %s (%v)", body, err)
	}
	target := links[0].Target
	if target.PackageName != "online.bmarian.bus" || target.SHA256CertFingerprints[0] != "AB:CD" ||
		links[0].Relation[0] != "delegate_permission/common.handle_all_urls" {
		t.Fatalf("link = %+v", links[0])
	}
}

func TestAssetLinksIsEmptyWithoutACertificate(t *testing.T) {
	app := fiber.New()
	app.Get("/x", AssetLinksHandler("online.bmarian.bus", nil))
	resp, _ := app.Test(httptest.NewRequest("GET", "/x", nil))
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "[]" {
		t.Fatalf("body = %s", body)
	}
}
