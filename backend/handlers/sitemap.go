package handlers

import (
	"conexiuni-cluj/database"
	"encoding/xml"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	sitemapBaseURL = "https://bus.bmarian.online"
	sitemapTTL     = 1 * time.Hour
)

var (
	sitemapMu       sync.RWMutex
	sitemapCache    []byte
	sitemapCachedAt time.Time
)

func SitemapHandler(c fiber.Ctx) error {
	sitemapMu.RLock()
	cached, cachedAt := sitemapCache, sitemapCachedAt
	sitemapMu.RUnlock()

	if cached != nil && time.Since(cachedAt) < sitemapTTL {
		c.Set("Content-Type", "application/xml; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(cached)
	}

	data, err := buildSitemap()
	if err != nil {
		if cached != nil {
			c.Set("Content-Type", "application/xml; charset=utf-8")
			c.Set("Cache-Control", "public, max-age=300")
			return c.Send(cached)
		}
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	sitemapMu.Lock()
	sitemapCache = data
	sitemapCachedAt = time.Now()
	sitemapMu.Unlock()

	c.Set("Content-Type", "application/xml; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=3600")
	return c.Send(data)
}

func buildSitemap() ([]byte, error) {
	var sb strings.Builder
	sb.Grow(64 * 1024)
	sb.WriteString(xml.Header)
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	writeURL(&sb, sitemapBaseURL+"/", "daily", "1.0")
	writeURL(&sb, sitemapBaseURL+"/plan", "monthly", "0.5")

	if err := writeRouteURLs(&sb); err != nil {
		return nil, err
	}
	if err := writeStopURLs(&sb); err != nil {
		return nil, err
	}

	sb.WriteString("</urlset>\n")
	return []byte(sb.String()), nil
}

func writeRouteURLs(sb *strings.Builder) error {
	rows, err := database.DB.Query(`
		SELECT DISTINCT r.route_id, t.direction_id, r.route_short_name
		FROM routes r
		INNER JOIN trips t ON t.route_id = r.route_id
		ORDER BY r.route_id, t.direction_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	availabilityReady := Availability.IsReady()
	for rows.Next() {
		var routeID, dirID int
		var shortName string
		if err := rows.Scan(&routeID, &dirID, &shortName); err != nil {
			return err
		}
		if availabilityReady && !Availability.RouteHasTimetable(shortName) {
			continue
		}
		writeURL(sb,
			sitemapBaseURL+"/route/"+strconv.Itoa(routeID)+"/"+strconv.Itoa(dirID),
			"weekly", "0.8")
	}
	return rows.Err()
}

func writeStopURLs(sb *strings.Builder) error {
	rows, err := database.DB.Query(`SELECT stop_id FROM stops ORDER BY stop_id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	availabilityReady := Availability.IsReady()
	for rows.Next() {
		var stopID int
		if err := rows.Scan(&stopID); err != nil {
			return err
		}
		if availabilityReady && !Availability.StopHasBuses(stopID) {
			continue
		}
		writeURL(sb,
			sitemapBaseURL+"/stop/"+strconv.Itoa(stopID),
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

// InvalidateSitemap clears the cached sitemap, forcing rebuild on next request.
// Called after warmup pass so newly-discovered routes/stops appear.
func InvalidateSitemap() {
	sitemapMu.Lock()
	sitemapCache = nil
	sitemapMu.Unlock()
}
