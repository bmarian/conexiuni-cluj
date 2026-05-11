package handlers

import (
	"conexiuni-cluj/database"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var (
	reTitleEl   = regexp.MustCompile(`(?i)<title>[^<]*</title>`)
	reMetaDesc  = regexp.MustCompile(`(?i)<meta\s+name="description"\s+content="[^"]*"`)
	reOGTitle   = regexp.MustCompile(`(?i)<meta\s+property="og:title"\s+content="[^"]*"`)
	reOGDesc    = regexp.MustCompile(`(?i)<meta\s+property="og:description"\s+content="[^"]*"`)
	reOGUrl     = regexp.MustCompile(`(?i)<meta\s+property="og:url"\s+content="[^"]*"`)
	reTWTitle   = regexp.MustCompile(`(?i)<meta\s+name="twitter:title"\s+content="[^"]*"`)
	reTWDesc    = regexp.MustCompile(`(?i)<meta\s+name="twitter:description"\s+content="[^"]*"`)
	reCanonical = regexp.MustCompile(`(?i)<link\s+rel="canonical"\s+href="[^"]*"\s*/?>`)
	reLangAttr  = regexp.MustCompile(`(<html[^>]*lang=")[^"]*(")`)
	reHeadClose = regexp.MustCompile(`(?i)</head>`)
	reAppMount  = regexp.MustCompile(`<div\s+id="app"\s*>\s*</div>`)
)

const siteOrigin = "https://bus.bmarian.online"

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
	routeDescFmt       string // args: shortName, longName
	routeH1Fmt         string // args: shortName
	routeBodyFmt       string // args: shortName, longName
	stopTitleFmt       string // args: stopName
	stopDescFmt        string // args: stopName
	stopH1Fmt          string // args: stopName
	stopBodyFmt        string // args: stopName
	planTitle          string
	planDesc           string
	planH1             string
	planBody           string
	planToTitleFmt     string // args: dest
	planToDescFmt      string // args: dest
	planFromToTitleFmt string // args: origin, dest
	planFromToDescFmt  string // args: origin, dest
	homeTitle          string
	homeDesc           string
	homeH1             string
	homeBody           string
}

var ogLocales = map[string]ogLocale{
	"ro": {
		lang:               "ro",
		routeTitleFmt:      "Orar Linia %s Cluj-Napoca — %s | Conexiuni Cluj",
		routeDescFmt:       "Orar, traseu, opriri și plecări live pentru linia %s (%s) din Cluj-Napoca.",
		routeH1Fmt:         "Orar Linia %s Cluj-Napoca",
		routeBodyFmt:       "Linia %s (%s) face parte din transportul public din Cluj-Napoca, operat de CTP Cluj-Napoca. Conexiuni Cluj este o aplicație care afișează orarele publice și pozițiile live ale vehiculelor.",
		stopTitleFmt:       "Stația %s, Cluj-Napoca — Plecări live | Conexiuni Cluj",
		stopDescFmt:        "Plecări în timp real de la stația %s din Cluj-Napoca. Liniile care opresc aici și orarele actuale.",
		stopH1Fmt:          "Stația %s, Cluj-Napoca",
		stopBodyFmt:        "Plecări în timp real de la stația %s din Cluj-Napoca. Vezi liniile de autobuz, troleibuz și tramvai care opresc aici, orele estimate și poziția vehiculelor pe hartă.",
		planTitle:          "Planificator rute — Conexiuni Cluj",
		planDesc:           "Planifică-ți călătoria cu transportul în comun din Cluj-Napoca. Găsește rute, orare și conexiuni între stații.",
		planH1:             "Planifică-ți călătoria în Cluj-Napoca",
		planBody:           "Planifică-ți ruta cu transportul în comun din Cluj-Napoca. Compară opțiuni, vezi conexiunile între linii și orele estimate de sosire.",
		planToTitleFmt:     "Rută la %s — Cluj-Napoca | Conexiuni Cluj",
		planToDescFmt:      "Planifică-ți călătoria la %s cu transportul în comun din Cluj-Napoca.",
		planFromToTitleFmt: "Rută de la %s la %s — Cluj-Napoca | Conexiuni Cluj",
		planFromToDescFmt:  "Planifică-ți călătoria de la %s la %s cu transportul în comun din Cluj-Napoca.",
		homeTitle:          "Conexiuni Cluj — Transport public Cluj-Napoca în timp real",
		homeDesc:           "Aplicație pentru transportul public din Cluj-Napoca. Orarele CTP Cluj-Napoca în timp real, plecările de la stații, traseele și planificator de rute.",
		homeH1:             "Transport public Cluj-Napoca în timp real",
		homeBody:           "Conexiuni Cluj este o aplicație care afișează în timp real autobuzele, troleibuzele și tramvaiele CTP Cluj-Napoca. Vezi orarele, plecările de la stații, traseele liniilor și planifică-ți călătoria.",
	},
	"en": {
		lang:               "en",
		routeTitleFmt:      "Line %s Timetable Cluj-Napoca — %s | Conexiuni Cluj",
		routeDescFmt:       "Timetable, route, stops and live departures for line %s (%s) in Cluj-Napoca.",
		routeH1Fmt:         "Line %s Timetable — Cluj-Napoca",
		routeBodyFmt:       "Line %s (%s) is part of Cluj-Napoca public transport, operated by CTP Cluj-Napoca. Conexiuni Cluj is an app showing public schedules and live vehicle positions.",
		stopTitleFmt:       "%s stop, Cluj-Napoca — Live departures | Conexiuni Cluj",
		stopDescFmt:        "Live departures from %s stop in Cluj-Napoca. Lines stopping here and current schedules.",
		stopH1Fmt:          "%s stop, Cluj-Napoca",
		stopBodyFmt:        "Live departures from %s in Cluj-Napoca. See bus, trolleybus and tram lines stopping here, estimated times and vehicle positions on the map.",
		planTitle:          "Route planner — Conexiuni Cluj",
		planDesc:           "Plan your journey with public transport in Cluj-Napoca. Find routes, timetables and connections.",
		planH1:             "Plan your journey in Cluj-Napoca",
		planBody:           "Plan your route with public transport in Cluj-Napoca. Compare options, see line connections and estimated arrival times.",
		planToTitleFmt:     "Trip to %s — Cluj-Napoca | Conexiuni Cluj",
		planToDescFmt:      "Plan your trip to %s with public transport in Cluj-Napoca.",
		planFromToTitleFmt: "Trip from %s to %s — Cluj-Napoca | Conexiuni Cluj",
		planFromToDescFmt:  "Plan your trip from %s to %s with public transport in Cluj-Napoca.",
		homeTitle:          "Conexiuni Cluj — Cluj-Napoca public transport in real time",
		homeDesc:           "App for Cluj-Napoca public transport. Real-time CTP Cluj-Napoca timetables, live departures, line routes and route planner.",
		homeH1:             "Cluj-Napoca public transport in real time",
		homeBody:           "Conexiuni Cluj is an app showing CTP Cluj-Napoca buses, trolleybuses and trams in real time. See timetables, stop departures, line routes and plan your trip.",
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

type metaInjection struct {
	title       string
	description string
	canonical   string // absolute URL of canonical version (Romanian, no lang query)
	bodyHTML    string // raw HTML inserted inside #app for crawlers; Vue replaces on mount
	jsonLD      string // raw JSON for application/ld+json (already JSON-encoded, no HTML escape)
}

func injectMeta(c fiber.Ctx, l ogLocale, m metaInjection) error {
	if indexHTMLContent == "" {
		return c.Next()
	}

	t := html.EscapeString(m.title)
	d := html.EscapeString(m.description)
	canonical := html.EscapeString(m.canonical)

	body := reTitleEl.ReplaceAllLiteralString(indexHTMLContent, "<title>"+t+"</title>")
	body = reMetaDesc.ReplaceAllLiteralString(body, `<meta name="description" content="`+d+`"`)
	body = reOGTitle.ReplaceAllLiteralString(body, `<meta property="og:title" content="`+t+`"`)
	body = reOGDesc.ReplaceAllLiteralString(body, `<meta property="og:description" content="`+d+`"`)
	body = reOGUrl.ReplaceAllLiteralString(body, `<meta property="og:url" content="`+canonical+`"`)
	body = reTWTitle.ReplaceAllLiteralString(body, `<meta name="twitter:title" content="`+t+`"`)
	body = reTWDesc.ReplaceAllLiteralString(body, `<meta name="twitter:description" content="`+d+`"`)
	body = reCanonical.ReplaceAllLiteralString(body, `<link rel="canonical" href="`+canonical+`">`)
	body = reLangAttr.ReplaceAllString(body, "${1}"+l.lang+"${2}")

	headExtras := buildHeadExtras(m.canonical, m.jsonLD)
	if headExtras != "" {
		body = reHeadClose.ReplaceAllLiteralString(body, headExtras+"</head>")
	}

	if m.bodyHTML != "" {
		body = reAppMount.ReplaceAllLiteralString(body, `<div id="app">`+m.bodyHTML+`</div>`)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=300")
	return c.SendString(body)
}

func buildHeadExtras(canonical, jsonLD string) string {
	var sb strings.Builder
	if canonical != "" {
		canonicalAttr := html.EscapeString(canonical)
		enHref := canonicalAttr
		if strings.Contains(canonical, "?") {
			enHref += "&lang=en"
		} else {
			enHref += "?lang=en"
		}
		sb.WriteString(`<link rel="alternate" hreflang="ro" href="` + canonicalAttr + `">`)
		sb.WriteString(`<link rel="alternate" hreflang="en" href="` + enHref + `">`)
		sb.WriteString(`<link rel="alternate" hreflang="x-default" href="` + canonicalAttr + `">`)
	}
	if jsonLD != "" {
		sb.WriteString(`<script type="application/ld+json">`)
		sb.WriteString(jsonLD)
		sb.WriteString(`</script>`)
	}
	return sb.String()
}

func canonicalURL(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return siteOrigin + path
}

func marshalJSONLD(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	// Prevent script-injection inside <script type="application/ld+json"> by escaping
	// any literal "</" sequences (the only way the script tag could be terminated early).
	return strings.ReplaceAll(string(b), "</", `<\/`)
}

func RouteOGHandler(c fiber.Ctx) error {
	routeID, err := strconv.Atoi(c.Params("routeId"))
	if err != nil {
		return c.Next()
	}
	direction, err := strconv.Atoi(c.Params("direction"))
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
	routeURL := canonicalURL("/route/" + strconv.Itoa(routeID) + "/" + strconv.Itoa(direction))
	title := fmt.Sprintf(l.routeTitleFmt, shortName, longName)
	desc := fmt.Sprintf(l.routeDescFmt, shortName, longName)
	jsonLD := marshalJSONLD(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebPage",
		"name":        title,
		"description": desc,
		"url":         routeURL,
		"inLanguage":  l.lang,
		"publisher":   publisherOrg(),
		"about": map[string]any{
			"@type":       "Service",
			"name":        "Linia " + shortName,
			"description": longName,
			"serviceType": "Public transport route",
			"areaServed": map[string]any{
				"@type": "City",
				"name":  "Cluj-Napoca",
			},
			"provider": map[string]any{
				"@type": "Organization",
				"name":  "Compania de Transport Public Cluj-Napoca",
			},
		},
	})

	return injectMeta(c, l, metaInjection{
		title:       title,
		description: desc,
		canonical:   routeURL,
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(fmt.Sprintf(l.routeH1Fmt, shortName)),
			html.EscapeString(fmt.Sprintf(l.routeBodyFmt, shortName, longName))),
		jsonLD: jsonLD,
	})
}

func publisherOrg() map[string]any {
	return map[string]any{
		"@type": "Organization",
		"name":  "Conexiuni Cluj",
		"url":   siteOrigin + "/",
	}
}

func StopOGHandler(c fiber.Ctx) error {
	stopID, err := strconv.Atoi(c.Params("stopId"))
	if err != nil {
		return c.Next()
	}

	var stopName string
	var stopLat, stopLon float64
	row := database.DB.QueryRow(
		`SELECT stop_name, stop_lat, stop_lon FROM stops WHERE stop_id = ?`,
		stopID,
	)
	if err := row.Scan(&stopName, &stopLat, &stopLon); err != nil {
		return c.Next()
	}

	l := detectLocale(c)
	stopURL := canonicalURL("/stop/" + strconv.Itoa(stopID))
	title := fmt.Sprintf(l.stopTitleFmt, stopName)
	desc := fmt.Sprintf(l.stopDescFmt, stopName)
	jsonLD := marshalJSONLD(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebPage",
		"name":        title,
		"description": desc,
		"url":         stopURL,
		"inLanguage":  l.lang,
		"publisher":   publisherOrg(),
		"about": map[string]any{
			"@type": "BusStop",
			"name":  stopName,
			"address": map[string]any{
				"@type":           "PostalAddress",
				"addressLocality": "Cluj-Napoca",
				"addressCountry":  "RO",
			},
			"geo": map[string]any{
				"@type":     "GeoCoordinates",
				"latitude":  stopLat,
				"longitude": stopLon,
			},
		},
	})

	return injectMeta(c, l, metaInjection{
		title:       title,
		description: desc,
		canonical:   stopURL,
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(fmt.Sprintf(l.stopH1Fmt, stopName)),
			html.EscapeString(fmt.Sprintf(l.stopBodyFmt, stopName))),
		jsonLD: jsonLD,
	})
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
		return injectMeta(c, l, metaInjection{
			title:       l.planTitle,
			description: l.planDesc,
			canonical:   canonicalURL("/plan"),
			bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
				html.EscapeString(l.planH1),
				html.EscapeString(l.planBody)),
			jsonLD: marshalJSONLD(map[string]any{
				"@context":    "https://schema.org",
				"@type":       "WebPage",
				"name":        l.planTitle,
				"description": l.planDesc,
				"url":         canonicalURL("/plan"),
				"inLanguage":  l.lang,
				"publisher":   publisherOrg(),
			}),
		})
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

	return injectMeta(c, l, metaInjection{
		title:       title,
		description: desc,
		canonical:   canonicalURL("/plan"),
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(title),
			html.EscapeString(desc)),
	})
}

func HomeOGHandler(c fiber.Ctx) error {
	l := detectLocale(c)
	return injectMeta(c, l, metaInjection{
		title:       l.homeTitle,
		description: l.homeDesc,
		canonical:   canonicalURL("/"),
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(l.homeH1),
			html.EscapeString(l.homeBody)),
		jsonLD: marshalJSONLD(map[string]any{
			"@context": "https://schema.org",
			"@type":    "WebSite",
			"name":     "Conexiuni Cluj",
			"url":      canonicalURL("/"),
			"potentialAction": map[string]any{
				"@type":       "SearchAction",
				"target":      canonicalURL("/?q={search_term_string}"),
				"query-input": "required name=search_term_string",
			},
		}),
	})
}
