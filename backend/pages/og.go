// Package pages serves the site's pages with their titles, descriptions and links filled in for each city, line and stop, and the sitemap.
package pages

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/web"
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
	androidTitle       string
	androidDesc        string
	androidBody        string
}

var ogLocales = map[string]ogLocale{
	"ro": {
		lang:               "ro",
		routeTitleFmt:      "Orar Linia %s {city} — %s | Ce autobuz iau?",
		routeDescFmt:       "Orar, traseu, opriri și plecări live pentru linia %s (%s) din {city}.",
		routeH1Fmt:         "Orar Linia %s {city}",
		routeBodyFmt:       "Linia %s (%s) face parte din transportul public din {city}, operat de {operator}. Aici găsești orarul public și pozițiile live ale vehiculelor.",
		stopTitleFmt:       "Stația %s, {city} — Plecări live | Ce autobuz iau?",
		stopDescFmt:        "Plecări în timp real de la stația %s din {city}. Liniile care opresc aici și orarele actuale.",
		stopH1Fmt:          "Stația %s, {city}",
		stopBodyFmt:        "Plecări în timp real de la stația %s din {city}. Vezi liniile de autobuz, troleibuz și tramvai care opresc aici, orele estimate și poziția vehiculelor pe hartă.",
		planTitle:          "Planificator rute — Ce autobuz iau?",
		planDesc:           "Planifică-ți călătoria cu transportul în comun din {city}. Găsește rute, orare și conexiuni între stații.",
		planH1:             "Planifică-ți călătoria în {city}",
		planBody:           "Planifică-ți ruta cu transportul în comun din {city}. Compară opțiuni, vezi conexiunile între linii și orele estimate de sosire.",
		planToTitleFmt:     "Rută la %s — {city} | Ce autobuz iau?",
		planToDescFmt:      "Planifică-ți călătoria la %s cu transportul în comun din {city}.",
		planFromToTitleFmt: "Rută de la %s la %s — {city} | Ce autobuz iau?",
		planFromToDescFmt:  "Planifică-ți călătoria de la %s la %s cu transportul în comun din {city}.",
		homeTitle:          "Ce autobuz iau? — Transport public {city} în timp real",
		homeDesc:           "Aplicație pentru transportul public din {city}. Orarele {operator} în timp real, plecările de la stații, traseele și planificator de rute.",
		homeH1:             "Transport public {city} în timp real",
		homeBody:           "Autobuzele, troleibuzele și tramvaiele {operator} în timp real. Vezi orarele, plecările de la stații, traseele liniilor și planifică-ți călătoria.",
		androidTitle:       "Aplicația Android — Ce autobuz iau?",
		androidDesc:        "Intră în beta și testează aplicația pentru telefon și ceas Wear OS.",
		androidBody:        "Intră în grupul de testeri cu contul Google, apoi ia aplicația din Google Play. Necesită Android 8 sau mai nou.",
	},
	"en": {
		lang:               "en",
		routeTitleFmt:      "Line %s Timetable {city} — %s | Ce autobuz iau?",
		routeDescFmt:       "Timetable, route, stops and live departures for line %s (%s) in {city}.",
		routeH1Fmt:         "Line %s Timetable — {city}",
		routeBodyFmt:       "Line %s (%s) is part of {city} public transport, operated by {operator}. Here you'll find its public timetable and live vehicle positions.",
		stopTitleFmt:       "%s stop, {city} — Live departures | Ce autobuz iau?",
		stopDescFmt:        "Live departures from %s stop in {city}. Lines stopping here and current schedules.",
		stopH1Fmt:          "%s stop, {city}",
		stopBodyFmt:        "Live departures from %s in {city}. See bus, trolleybus and tram lines stopping here, estimated times and vehicle positions on the map.",
		planTitle:          "Route planner — Ce autobuz iau?",
		planDesc:           "Plan your journey with public transport in {city}. Find routes, timetables and connections.",
		planH1:             "Plan your journey in {city}",
		planBody:           "Plan your route with public transport in {city}. Compare options, see line connections and estimated arrival times.",
		planToTitleFmt:     "Trip to %s — {city} | Ce autobuz iau?",
		planToDescFmt:      "Plan your trip to %s with public transport in {city}.",
		planFromToTitleFmt: "Trip from %s to %s — {city} | Ce autobuz iau?",
		planFromToDescFmt:  "Plan your trip from %s to %s with public transport in {city}.",
		homeTitle:          "Ce autobuz iau? — {city} public transport in real time",
		homeDesc:           "App for {city} public transport. Real-time {operator} timetables, live departures, line routes and route planner.",
		homeH1:             "{city} public transport in real time",
		homeBody:           "{operator} buses, trolleybuses and trams in real time. See timetables, stop departures, line routes and plan your trip.",
		androidTitle:       "Android app — Ce autobuz iau?",
		androidDesc:        "Join the beta to test the app for your phone and Wear OS watch.",
		androidBody:        "Join the testers group with your Google account, then get the app from Google Play. Needs Android 8 or newer.",
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
	l, ok := ogLocales[lang]
	if !ok {
		l = ogLocales["ro"]
	}
	return l
}

func ogTexts(city *cities.City, c fiber.Ctx) ogLocale {
	l := detectLocale(c)
	fill := strings.NewReplacer("{city}", city.Name, "{operator}", city.Operator).Replace
	l.routeTitleFmt = fill(l.routeTitleFmt)
	l.routeDescFmt = fill(l.routeDescFmt)
	l.routeH1Fmt = fill(l.routeH1Fmt)
	l.routeBodyFmt = fill(l.routeBodyFmt)
	l.stopTitleFmt = fill(l.stopTitleFmt)
	l.stopDescFmt = fill(l.stopDescFmt)
	l.stopH1Fmt = fill(l.stopH1Fmt)
	l.stopBodyFmt = fill(l.stopBodyFmt)
	l.planDesc = fill(l.planDesc)
	l.planH1 = fill(l.planH1)
	l.planBody = fill(l.planBody)
	l.planToTitleFmt = fill(l.planToTitleFmt)
	l.planToDescFmt = fill(l.planToDescFmt)
	l.planFromToTitleFmt = fill(l.planFromToTitleFmt)
	l.planFromToDescFmt = fill(l.planFromToDescFmt)
	l.homeTitle = fill(l.homeTitle)
	l.homeDesc = fill(l.homeDesc)
	l.homeH1 = fill(l.homeH1)
	l.homeBody = fill(l.homeBody)
	return l
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
	canonical := html.EscapeString(selfURL(c, m.canonical))

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
		body = reAppMount.ReplaceAllLiteralString(body, `<div id="app"><div style="position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)">`+m.bodyHTML+`</div></div>`)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=300")
	return c.SendString(body)
}

func buildHeadExtras(canonical, jsonLD string) string {
	var sb strings.Builder
	if canonical != "" {
		canonicalAttr := html.EscapeString(canonical)
		enHref := html.EscapeString(withLang(canonical, "en"))
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

// selfURL is the page's own canonical: a ?lang=en page is the hreflang="en" alternate, not a copy of the Romanian one.
func selfURL(c fiber.Ctx, canonical string) string {
	if c.Query("lang") == "en" {
		return withLang(canonical, "en")
	}
	return canonical
}

func withLang(u, lang string) string {
	if strings.Contains(u, "?") {
		return u + "&lang=" + lang
	}
	return u + "?lang=" + lang
}

func canonicalURL(city *cities.City, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return web.SiteOrigin + "/" + city.Slug + path
}

func marshalJSONLD(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	// Escape "</" so the JSON can't close the script tag.
	return strings.ReplaceAll(string(b), "</", `<\/`)
}

func routePage(city *cities.City, c fiber.Ctx) error {
	routeID, err := strconv.Atoi(c.Params("routeId"))
	if err != nil {
		return c.Next()
	}
	direction, err := strconv.Atoi(c.Params("direction"))
	if err != nil {
		return c.Next()
	}

	routes, err := city.Routes(transit.RouteFilter{RouteID: &routeID})
	if err != nil || len(routes) == 0 {
		return c.Next()
	}
	shortName, longName := routes[0].RouteShortName, routes[0].RouteLongName

	l := ogTexts(city, c)
	routeURL := canonicalURL(city, "/route/"+strconv.Itoa(routeID)+"/"+strconv.Itoa(direction))
	title := fmt.Sprintf(l.routeTitleFmt, shortName, longName)
	desc := fmt.Sprintf(l.routeDescFmt, shortName, longName)
	jsonLD := marshalJSONLD(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebPage",
		"name":        title,
		"description": desc,
		"url":         selfURL(c, routeURL),
		"inLanguage":  l.lang,
		"publisher":   publisherOrg(),
		"about": map[string]any{
			"@type":       "Service",
			"name":        "Linia " + shortName,
			"description": longName,
			"serviceType": "Public transport route",
			"areaServed": map[string]any{
				"@type": "City",
				"name":  city.Name,
			},
			"provider": map[string]any{
				"@type": "Organization",
				"name":  city.Operator,
				"url":   city.OperatorURL,
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
		"name":  "Ce autobuz iau?",
		"url":   web.SiteOrigin + "/",
	}
}

func stopPage(city *cities.City, c fiber.Ctx) error {
	stopID, err := strconv.Atoi(c.Params("stopId"))
	if err != nil {
		return c.Next()
	}

	stops, err := city.Stops(transit.StopFilter{StopID: &stopID})
	if err != nil || len(stops) == 0 {
		return c.Next()
	}
	stopName, stopLat, stopLon := stops[0].StopName, stops[0].StopLat, stops[0].StopLon

	l := ogTexts(city, c)
	stopURL := canonicalURL(city, "/stop/"+strconv.Itoa(stopID))
	title := fmt.Sprintf(l.stopTitleFmt, stopName)
	desc := fmt.Sprintf(l.stopDescFmt, stopName)
	jsonLD := marshalJSONLD(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebPage",
		"name":        title,
		"description": desc,
		"url":         selfURL(c, stopURL),
		"inLanguage":  l.lang,
		"publisher":   publisherOrg(),
		"about": map[string]any{
			"@type": "BusStop",
			"name":  stopName,
			"address": map[string]any{
				"@type":           "PostalAddress",
				"addressLocality": city.Name,
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

func shortPlaceName(name string) string {
	if i := strings.IndexByte(name, ','); i > 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}

func planPage(city *cities.City, c fiber.Ctx) error {
	l := ogTexts(city, c)

	dest := c.Query("name")
	if dest == "" {
		return injectMeta(c, l, metaInjection{
			title:       l.planTitle,
			description: l.planDesc,
			canonical:   canonicalURL(city, "/plan"),
			bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
				html.EscapeString(l.planH1),
				html.EscapeString(l.planBody)),
			jsonLD: marshalJSONLD(map[string]any{
				"@context":    "https://schema.org",
				"@type":       "WebPage",
				"name":        l.planTitle,
				"description": l.planDesc,
				"url":         selfURL(c, canonicalURL(city, "/plan")),
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
		canonical:   canonicalURL(city, "/plan"),
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(title),
			html.EscapeString(desc)),
	})
}

func homePage(city *cities.City, c fiber.Ctx) error {
	l := ogTexts(city, c)
	return injectMeta(c, l, metaInjection{
		title:       l.homeTitle,
		description: l.homeDesc,
		canonical:   canonicalURL(city, "/"),
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(l.homeH1),
			html.EscapeString(l.homeBody)),
		jsonLD: marshalJSONLD(map[string]any{
			"@context": "https://schema.org",
			"@type":    "WebSite",
			"name":     "Ce autobuz iau?",
			"url":      canonicalURL(city, "/"),
		}),
	})
}

func Android(c fiber.Ctx) error {
	l := detectLocale(c)
	return injectMeta(c, l, metaInjection{
		title:       l.androidTitle,
		description: l.androidDesc,
		canonical:   web.SiteOrigin + "/android",
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(l.androidTitle),
			html.EscapeString(l.androidBody)),
		jsonLD: marshalJSONLD(map[string]any{
			"@context":    "https://schema.org",
			"@type":       "WebPage",
			"name":        l.androidTitle,
			"description": l.androidDesc,
			"url":         selfURL(c, web.SiteOrigin+"/android"),
			"inLanguage":  l.lang,
			"publisher":   publisherOrg(),
		}),
	})
}
