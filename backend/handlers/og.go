package handlers

import (
	"conexiuni-cluj/database"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var crawlerUserAgents = []string{
	"Discordbot",
	"Twitterbot",
	"facebookexternalhit",
	"LinkedInBot",
	"Slackbot-LinkExpanding",
	"TelegramBot",
	"WhatsApp",
	"Googlebot",
	"bingbot",
	"Applebot",
	"iMessage",
	"Iframely",
}

func isCrawler(ua string) bool {
	for _, bot := range crawlerUserAgents {
		if strings.Contains(ua, bot) {
			return true
		}
	}
	return false
}

type ogLocale struct {
	routeTitleFmt string // args: shortName, longName
	routeDescFmt  string // args: shortName
	stopTitleFmt  string // args: stopName
	stopDescFmt   string // args: stopName
	planTitle     string
	planDesc      string
}

var ogLocales = map[string]ogLocale{
	"ro": {
		routeTitleFmt: "Linia %s — %s | Conexiuni Cluj",
		routeDescFmt:  "Traseu, opriri și orare pentru linia %s din Cluj-Napoca.",
		stopTitleFmt:  "Stația %s | Conexiuni Cluj",
		stopDescFmt:   "Plecări în timp real de la stația %s.",
		planTitle:     "Planifică-ți ruta - Conexiuni Cluj",
		planDesc:      "Planifică-ți călătoria cu transportul în comun în Cluj-Napoca. Găsește rute, orare și conexiuni.",
	},
	"en": {
		routeTitleFmt: "Line %s — %s | Conexiuni Cluj",
		routeDescFmt:  "Route, stops and timetables for line %s in Cluj-Napoca.",
		stopTitleFmt:  "Stop %s | Conexiuni Cluj",
		stopDescFmt:   "Real-time departures from stop %s.",
		planTitle:     "Plan your route - Conexiuni Cluj",
		planDesc:      "Plan your journey with public transport in Cluj-Napoca. Find routes, timetables and connections.",
	},
}

func detectLocale(c fiber.Ctx) ogLocale {
	lang := c.Query("lang")
	if lang == "" {
		for _, part := range strings.Split(c.Get("Accept-Language"), ",") {
			tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
			if strings.HasPrefix(tag, "en") {
				lang = "en"
				break
			}
			if strings.HasPrefix(tag, "ro") {
				lang = "ro"
				break
			}
		}
	}
	if l, ok := ogLocales[lang]; ok {
		return l
	}
	return ogLocales["ro"]
}

func ogResponse(c fiber.Ctx, title, description string) error {
	base := c.Protocol() + "://" + c.Hostname()
	u := html.EscapeString(base + c.OriginalURL())
	img := html.EscapeString(base + "/pwa-512x512.png")
	t := html.EscapeString(title)
	d := html.EscapeString(description)

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=300")
	return c.SendString(`<!DOCTYPE html>` + "\n" +
		`<html lang="ro"><head>` + "\n" +
		`<meta charset="UTF-8">` + "\n" +
		`<title>` + t + `</title>` + "\n" +
		`<meta name="description" content="` + d + `">` + "\n" +
		`<meta property="og:type" content="website">` + "\n" +
		`<meta property="og:site_name" content="Conexiuni Cluj">` + "\n" +
		`<meta property="og:title" content="` + t + `">` + "\n" +
		`<meta property="og:description" content="` + d + `">` + "\n" +
		`<meta property="og:url" content="` + u + `">` + "\n" +
		`<meta property="og:image" content="` + img + `">` + "\n" +
		`<meta name="twitter:card" content="summary">` + "\n" +
		`<meta name="twitter:title" content="` + t + `">` + "\n" +
		`<meta name="twitter:description" content="` + d + `">` + "\n" +
		`<meta name="twitter:image" content="` + img + `">` + "\n" +
		`</head><body></body></html>`,
	)
}

func RouteOGHandler(c fiber.Ctx) error {
	if !isCrawler(c.Get("User-Agent")) {
		return c.Next()
	}

	routeID, err := strconv.Atoi(c.Params("routeId"))
	if err != nil {
		return c.Next()
	}

	var shortName, longName string
	row := database.DB.QueryRow(
		`SELECT route_short_name, route_long_name FROM routes WHERE route_id = ?`,
		routeID,
	)
	if err := row.Scan(&shortName, &longName); err != nil {
		return c.Next()
	}

	l := detectLocale(c)
	return ogResponse(c,
		fmt.Sprintf(l.routeTitleFmt, shortName, longName),
		fmt.Sprintf(l.routeDescFmt, shortName),
	)
}

func StopOGHandler(c fiber.Ctx) error {
	if !isCrawler(c.Get("User-Agent")) {
		return c.Next()
	}

	stopID, err := strconv.Atoi(c.Params("stopId"))
	if err != nil {
		return c.Next()
	}

	var stopName string
	row := database.DB.QueryRow(
		`SELECT stop_name FROM stops WHERE stop_id = ?`,
		stopID,
	)
	if err := row.Scan(&stopName); err != nil {
		return c.Next()
	}

	l := detectLocale(c)
	return ogResponse(c,
		fmt.Sprintf(l.stopTitleFmt, stopName),
		fmt.Sprintf(l.stopDescFmt, stopName),
	)
}

func PlanOGHandler(c fiber.Ctx) error {
	if !isCrawler(c.Get("User-Agent")) {
		return c.Next()
	}
	l := detectLocale(c)
	return ogResponse(c, l.planTitle, l.planDesc)
}
