package handlers

import (
	"conexiuni-cluj/database"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var (
	reTitleEl  = regexp.MustCompile(`(?i)<title>[^<]*</title>`)
	reMetaDesc = regexp.MustCompile(`(?i)<meta\s+name="description"\s+content="[^"]*"`)
	reOGTitle  = regexp.MustCompile(`(?i)<meta\s+property="og:title"\s+content="[^"]*"`)
	reOGDesc   = regexp.MustCompile(`(?i)<meta\s+property="og:description"\s+content="[^"]*"`)
	reOGUrl    = regexp.MustCompile(`(?i)<meta\s+property="og:url"\s+content="[^"]*"`)
	reTWTitle  = regexp.MustCompile(`(?i)<meta\s+name="twitter:title"\s+content="[^"]*"`)
	reTWDesc   = regexp.MustCompile(`(?i)<meta\s+name="twitter:description"\s+content="[^"]*"`)
	reLangAttr = regexp.MustCompile(`(<html[^>]*lang=")[^"]*(")`)
)

var indexHTMLContent string

func LoadIndexHTML() {
	data, err := os.ReadFile("./dist/index.html")
	if err != nil {
		return
	}
	indexHTMLContent = string(data)
}

type ogLocale struct {
	lang               string
	routeTitleFmt      string // args: shortName, longName
	routeDescFmt       string // args: shortName
	stopTitleFmt       string // args: stopName
	stopDescFmt        string // args: stopName
	planTitle          string
	planDesc           string
	planToTitleFmt     string // args: dest
	planToDescFmt      string // args: dest
	planFromToTitleFmt string // args: origin, dest
	planFromToDescFmt  string // args: origin, dest
}

var ogLocales = map[string]ogLocale{
	"ro": {
		lang:               "ro",
		routeTitleFmt:      "Linia %s — %s | Conexiuni Cluj",
		routeDescFmt:       "Traseu, opriri și orare pentru linia %s din Cluj-Napoca.",
		stopTitleFmt:       "Stația %s | Conexiuni Cluj",
		stopDescFmt:        "Plecări în timp real de la stația %s.",
		planTitle:          "Planifică-ți ruta - Conexiuni Cluj",
		planDesc:           "Planifică-ți călătoria cu transportul în comun în Cluj-Napoca. Găsește rute, orare și conexiuni.",
		planToTitleFmt:     "Rută la %s | Conexiuni Cluj",
		planToDescFmt:      "Planifică-ți călătoria la %s cu transportul în comun din Cluj-Napoca.",
		planFromToTitleFmt: "Rută de la %s la %s | Conexiuni Cluj",
		planFromToDescFmt:  "Planifică-ți călătoria de la %s la %s cu transportul în comun din Cluj-Napoca.",
	},
	"en": {
		lang:               "en",
		routeTitleFmt:      "Line %s — %s | Conexiuni Cluj",
		routeDescFmt:       "Route, stops and timetables for line %s in Cluj-Napoca.",
		stopTitleFmt:       "Stop %s | Conexiuni Cluj",
		stopDescFmt:        "Real-time departures from stop %s.",
		planTitle:          "Plan your route - Conexiuni Cluj",
		planDesc:           "Plan your journey with public transport in Cluj-Napoca. Find routes, timetables and connections.",
		planToTitleFmt:     "Trip to %s | Conexiuni Cluj",
		planToDescFmt:      "Plan your trip to %s with public transport in Cluj-Napoca.",
		planFromToTitleFmt: "Trip from %s to %s | Conexiuni Cluj",
		planFromToDescFmt:  "Plan your trip from %s to %s with public transport in Cluj-Napoca.",
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

func injectMeta(c fiber.Ctx, l ogLocale, title, description string) error {
	if indexHTMLContent == "" {
		return c.Next()
	}

	scheme := c.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = c.Protocol()
	}
	u := html.EscapeString(scheme + "://" + c.Hostname() + c.OriginalURL())
	t := html.EscapeString(title)
	d := html.EscapeString(description)

	body := reTitleEl.ReplaceAllLiteralString(indexHTMLContent, "<title>"+t+"</title>")
	body = reMetaDesc.ReplaceAllLiteralString(body, `<meta name="description" content="`+d+`"`)
	body = reOGTitle.ReplaceAllLiteralString(body, `<meta property="og:title" content="`+t+`"`)
	body = reOGDesc.ReplaceAllLiteralString(body, `<meta property="og:description" content="`+d+`"`)
	body = reOGUrl.ReplaceAllLiteralString(body, `<meta property="og:url" content="`+u+`"`)
	body = reTWTitle.ReplaceAllLiteralString(body, `<meta name="twitter:title" content="`+t+`"`)
	body = reTWDesc.ReplaceAllLiteralString(body, `<meta name="twitter:description" content="`+d+`"`)
	body = reLangAttr.ReplaceAllString(body, "${1}"+l.lang+"${2}")

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=300")
	return c.SendString(body)
}

func RouteOGHandler(c fiber.Ctx) error {
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
	return injectMeta(c, l,
		fmt.Sprintf(l.routeTitleFmt, shortName, longName),
		fmt.Sprintf(l.routeDescFmt, shortName),
	)
}

func StopOGHandler(c fiber.Ctx) error {
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
	return injectMeta(c, l,
		fmt.Sprintf(l.stopTitleFmt, stopName),
		fmt.Sprintf(l.stopDescFmt, stopName),
	)
}

// shortPlaceName returns the first comma-delimited segment of a geocoded address.
// "The Office Wine Bar, 77, Bulevardul 21 Decembrie..." → "The Office Wine Bar"
func shortPlaceName(name string) string {
	if i := strings.IndexByte(name, ','); i > 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}

func PlanOGHandler(c fiber.Ctx) error {
	l := detectLocale(c)

	dest := c.Query("name")
	if dest == "" {
		return injectMeta(c, l, l.planTitle, l.planDesc)
	}

	shortDest := shortPlaceName(dest)
	origin := c.Query("originName")

	var title, desc string
	if origin != "" {
		shortOrigin := shortPlaceName(origin)
		title = fmt.Sprintf(l.planFromToTitleFmt, shortOrigin, shortDest)
		desc = fmt.Sprintf(l.planFromToDescFmt, shortOrigin, shortDest)
	} else {
		title = fmt.Sprintf(l.planToTitleFmt, shortDest)
		desc = fmt.Sprintf(l.planToDescFmt, shortDest)
	}

	return injectMeta(c, l, title, desc)
}
