package pages

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/web"
	"encoding/xml"
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
		body, complete := cached, true
		if version := registry.Version(); cached == nil || time.Since(cachedAt) >= sitemapTTL || version != cachedVersion {
			var err error
			if body, complete, err = buildSitemap(registry); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
			}
			if complete {
				cached, cachedAt, cachedVersion = body, time.Now(), version
			}
		}
		c.Set("Content-Type", "application/xml; charset=utf-8")
		if complete {
			c.Set("Cache-Control", "public, max-age=3600")
		} else {
			c.Set("Cache-Control", "no-cache")
		}
		return c.Send(body)
	}
}

func buildSitemap(registry *cities.Registry) (body []byte, complete bool, err error) {
	var sb strings.Builder
	sb.Grow(64 * 1024)
	sb.WriteString(xml.Header)
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	writeURL(&sb, web.SiteOrigin+"/", "daily", "1.0")
	complete = true
	for _, city := range registry.Enabled() {
		ready, err := writeCity(&sb, city)
		if err != nil {
			return nil, false, err
		}
		complete = complete && ready
	}
	sb.WriteString("</urlset>\n")
	return []byte(sb.String()), complete, nil
}

func writeCity(sb *strings.Builder, city *cities.City) (bool, error) {
	writeURL(sb, canonicalURL(city, "/"), "daily", "1.0")
	writeURL(sb, canonicalURL(city, "/plan"), "monthly", "0.5")
	offered := city.Availability()
	routes, err := city.RouteDirections()
	if err != nil {
		return false, err
	}
	written := map[string]bool{}
	once := func(path, changeFreq, priority string) {
		if !written[path] {
			written[path] = true
			writeURL(sb, canonicalURL(city, path), changeFreq, priority)
		}
	}
	for _, r := range routes {
		if offered.IsReady() && !offered.RouteHasTimetable(r.RouteShortName) {
			continue
		}
		once(lineCanonicalPath(city, r.RouteID, r.DirectionID), "weekly", "0.8")
	}
	stops, err := city.Stops(transit.StopFilter{})
	if err != nil {
		return false, err
	}
	for _, s := range stops {
		if offered.IsReady() && !offered.StopHasBuses(s.StopID) {
			continue
		}
		once(stationPath(city, s.Station()), "weekly", "0.6")
	}
	x, ready, err := city.Destinations.Areas()
	if err != nil || !ready {
		return false, nil
	}
	writeURL(sb, canonicalURL(city, destinationsPath), "weekly", "0.7")
	for _, d := range x.Destinations() {
		writeURL(sb, canonicalURL(city, areaPath(d.Slug)), "weekly", "0.8")
	}
	return offered.IsReady(), nil
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
