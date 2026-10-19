package handlers

import (
	"encoding/xml"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const sitemapTTL = 1 * time.Hour

func SitemapHandler(cities *Cities) fiber.Handler {
	var mu sync.Mutex
	var cached []byte
	var cachedAt time.Time
	var cachedVersion int64
	return func(c fiber.Ctx) error {
		mu.Lock()
		defer mu.Unlock()
		if version := cities.version.Load(); cached == nil || time.Since(cachedAt) >= sitemapTTL || version != cachedVersion {
			cachedVersion = version
			var sb strings.Builder
			sb.Grow(64 * 1024)
			sb.WriteString(xml.Header)
			sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
			writeURL(&sb, siteOrigin+"/", "daily", "1.0")
			writeURL(&sb, siteOrigin+"/android", "monthly", "0.5")
			for _, city := range cities.Enabled() {
				if err := city.writeSitemap(&sb); err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
				}
			}
			sb.WriteString("</urlset>\n")
			cached, cachedAt = []byte(sb.String()), time.Now()
		}
		c.Set("Content-Type", "application/xml; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(cached)
	}
}

func (city *City) writeSitemap(sb *strings.Builder) error {
	writeURL(sb, city.canonicalURL("/"), "daily", "1.0")
	writeURL(sb, city.canonicalURL("/plan"), "monthly", "0.5")
	if err := city.writeRouteURLs(sb); err != nil {
		return err
	}
	return city.writeStopURLs(sb)
}

func (city *City) writeRouteURLs(sb *strings.Builder) error {
	rows, err := city.DB.Query(`
		SELECT DISTINCT r.route_id, t.direction_id, r.route_short_name
		FROM routes r
		INNER JOIN trips t ON t.route_id = r.route_id
		ORDER BY r.route_id, t.direction_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	availabilityReady := city.availability.IsReady()
	for rows.Next() {
		var routeID, dirID int
		var shortName string
		if err := rows.Scan(&routeID, &dirID, &shortName); err != nil {
			return err
		}
		if availabilityReady && !city.availability.RouteHasTimetable(shortName) {
			continue
		}
		writeURL(sb,
			city.canonicalURL("/route/"+strconv.Itoa(routeID)+"/"+strconv.Itoa(dirID)),
			"weekly", "0.8")
	}
	return rows.Err()
}

func (city *City) writeStopURLs(sb *strings.Builder) error {
	rows, err := city.DB.Query(`SELECT stop_id FROM stops ORDER BY stop_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	availabilityReady := city.availability.IsReady()
	for rows.Next() {
		var stopID int
		if err := rows.Scan(&stopID); err != nil {
			return err
		}
		if availabilityReady && !city.availability.StopHasBuses(stopID) {
			continue
		}
		writeURL(sb,
			city.canonicalURL("/stop/"+strconv.Itoa(stopID)),
			"weekly", "0.6")
	}
	return rows.Err()
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
