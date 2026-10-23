package pages

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/web"
	"encoding/xml"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const sitemapTTL = 1 * time.Hour

func Sitemap(registry *cities.Registry) fiber.Handler {
	var mu sync.Mutex
	var cached []byte
	var cachedAt time.Time
	var cachedVersion int64
	return func(c fiber.Ctx) error {
		mu.Lock()
		defer mu.Unlock()
		if version := registry.Version(); cached == nil || time.Since(cachedAt) >= sitemapTTL || version != cachedVersion {
			var sb strings.Builder
			sb.Grow(64 * 1024)
			sb.WriteString(xml.Header)
			sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
			writeURL(&sb, web.SiteOrigin+"/", "daily", "1.0")
			writeURL(&sb, web.SiteOrigin+"/android", "monthly", "0.5")
			for _, city := range registry.Enabled() {
				if err := writeCity(&sb, city); err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
				}
			}
			sb.WriteString("</urlset>\n")
			cached, cachedAt, cachedVersion = []byte(sb.String()), time.Now(), version
		}
		c.Set("Content-Type", "application/xml; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(cached)
	}
}

func writeCity(sb *strings.Builder, city *cities.City) error {
	writeURL(sb, canonicalURL(city, "/"), "daily", "1.0")
	writeURL(sb, canonicalURL(city, "/plan"), "monthly", "0.5")
	offered := city.Availability()
	routes, err := city.RouteDirections()
	if err != nil {
		return err
	}
	for _, r := range routes {
		if offered.IsReady() && !offered.RouteHasTimetable(r.RouteShortName) {
			continue
		}
		writeURL(sb, canonicalURL(city, "/route/"+strconv.Itoa(r.RouteID)+"/"+strconv.Itoa(r.DirectionID)), "weekly", "0.8")
	}
	stops, err := city.StopIDs()
	if err != nil {
		return err
	}
	for _, id := range stops {
		if offered.IsReady() && !offered.StopHasBuses(id) {
			continue
		}
		writeURL(sb, canonicalURL(city, "/stop/"+strconv.Itoa(id)), "weekly", "0.6")
	}
	if x, ready, err := city.Destinations.Areas(); err == nil && ready {
		writeURL(sb, canonicalURL(city, "/destinations"), "weekly", "0.7")
		for _, d := range x.Destinations() {
			writeURL(sb, canonicalURL(city, "/to/"+d.Slug), "weekly", "0.8")
		}
	}
	return nil
}

func writeURL(sb *strings.Builder, loc, changeFreq, priority string) {
	sb.WriteString("  <url><loc>")
	sb.WriteString(loc)
	sb.WriteString("</loc>")
	if changeFreq != "" {
		sb.WriteString("<changefreq>")
		sb.WriteString(changeFreq)
		sb.WriteString("</changefreq>")
	}
	if priority != "" {
		sb.WriteString("<priority>")
		sb.WriteString(priority)
		sb.WriteString("</priority>")
	}
	sb.WriteString("</url>\n")
}
