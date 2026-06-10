package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

var (
	allowedGMapsPattern = regexp.MustCompile(`^https?://maps\.app\.goo\.gl/[A-Za-z0-9_-]+$`)
	gmapsPinCoord       = regexp.MustCompile(`!3d(-?\d+\.\d+)!4d(-?\d+\.\d+)`)
	gmapsAtCoord        = regexp.MustCompile(`/@(-?\d+\.\d+),(-?\d+\.\d+)`)
	gmapsQueryCoord     = regexp.MustCompile(`[?&]q=(-?\d+\.\d+),(-?\d+\.\d+)`)
	gmapsPlaceName      = regexp.MustCompile(`/maps/place/([^/@?#]+)`)
)

func extractGoogleMapsCoords(rawURL string) (lat, lon float64, label string, err error) {
	// !3d!4d is the actual pinned location — more accurate than the camera @lat,lon
	if m := gmapsPinCoord.FindStringSubmatch(rawURL); m != nil {
		if lat, err = strconv.ParseFloat(m[1], 64); err != nil {
			return
		}
		if lon, err = strconv.ParseFloat(m[2], 64); err != nil {
			return
		}
		if mp := gmapsPlaceName.FindStringSubmatch(rawURL); mp != nil {
			if decoded, decErr := url.QueryUnescape(strings.ReplaceAll(mp[1], "+", " ")); decErr == nil {
				label = decoded
			}
		}
		if label == "" {
			label = fmt.Sprintf("%.4f, %.4f", lat, lon)
		}
		return
	}

	// Camera center fallback (@lat,lon)
	if m := gmapsAtCoord.FindStringSubmatch(rawURL); m != nil {
		if lat, err = strconv.ParseFloat(m[1], 64); err != nil {
			return
		}
		if lon, err = strconv.ParseFloat(m[2], 64); err != nil {
			return
		}
		if mp := gmapsPlaceName.FindStringSubmatch(rawURL); mp != nil {
			if decoded, decErr := url.QueryUnescape(strings.ReplaceAll(mp[1], "+", " ")); decErr == nil {
				label = decoded
			}
		}
		if label == "" {
			label = fmt.Sprintf("%.4f, %.4f", lat, lon)
		}
		return
	}

	// ?q=lat,lon fallback
	if m := gmapsQueryCoord.FindStringSubmatch(rawURL); m != nil {
		if lat, err = strconv.ParseFloat(m[1], 64); err != nil {
			return
		}
		if lon, err = strconv.ParseFloat(m[2], 64); err != nil {
			return
		}
		label = fmt.Sprintf("%.4f, %.4f", lat, lon)
		return
	}

	err = fmt.Errorf("no coordinates found in URL")
	return
}

func ResolveLocationHandler(c fiber.Ctx) error {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url required"})
	}
	if !allowedGMapsPattern.MatchString(rawURL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "only maps.app.goo.gl links are supported"})
	}

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(c.Context(), http.MethodHead, rawURL, nil)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid url"})
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ConexiuniCluj/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to resolve link"})
	}
	defer resp.Body.Close()

	targetURL := rawURL
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		if loc := resp.Header.Get("Location"); loc != "" {
			targetURL = loc
		}
	}

	lat, lon, label, err := extractGoogleMapsCoords(targetURL)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "no coordinates found in link"})
	}

	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{
		"lat":   lat,
		"lon":   lon,
		"label": label,
	})
}
