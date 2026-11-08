package pages

import (
	"conexiuni-cluj/cities"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSourcesLinksEveryOperator(t *testing.T) {
	city, dir := testCity(t)
	city.Operator, city.OperatorURL = "Transit A", "https://a.example/"
	registry, err := cities.NewRegistry([]*cities.City{city}, filepath.Join(dir, "disabled-cities.json"))
	if err != nil {
		t.Fatal(err)
	}
	saved := indexHTMLContent
	indexHTMLContent = `<html lang="ro"><head><title>x</title></head><body><div id="app"></div></body></html>`
	t.Cleanup(func() { indexHTMLContent = saved })
	app := fiber.New()
	app.Get(SourcesPath, Sources(registry))
	for lang, title := range map[string]string{"": "Surse", "en": "Sources"} {
		res, err := app.Test(httptest.NewRequest("GET", SourcesPath+"?lang="+lang, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		page := string(body)
		if res.StatusCode != 200 || !strings.Contains(page, "<title>"+title) || !strings.Contains(page, `A: <a href="https://a.example/">Transit A</a>`) {
			t.Errorf("lang %q got %d: %s", lang, res.StatusCode, page)
		}
	}
}
