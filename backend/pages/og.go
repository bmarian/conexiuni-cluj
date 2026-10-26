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
	"slices"
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
	routeStopsFmt      string // args: shortName, toward
	stopLines          string
	towardFmt          string // args: end
	lineEnds           string
	stopAreaFmt        string // args: area
	destTitleFmt       string // args: area
	destDescFmt        string // args: area
	destH1Fmt          string // args: area
	destLinesFmt       string // args: area, lines
	destFrom           string
	destFromFmt        string // args: area
	rideFmt            string // args: shortName, board, alight, minutes
	rideEveryFmt       string // args: minutes
	rideSpanFmt        string // args: first, last
	rideNoWeekdays     string
	destNoRides        string
	destStopsFmt       string // args: area
	destPlanFmt        string // args: area
	destAll            string
	allLines           string
	lineItemFmt        string // args: shortName, longName
	destsTitle         string
	destsDesc          string
	destsH1            string
	destsDistricts     string
	destsTowns         string
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
		homeTitle:          "Mersul autobuzelor {city} — orar în timp real | Ce autobuz iau?",
		homeDesc:           "Orarul autobuzelor din {city} în timp real: plecări live din stații, orarul și traseul fiecărei linii {operator} și planificator de rute.",
		homeH1:             "Transport public {city} în timp real",
		homeBody:           "Autobuzele, troleibuzele și tramvaiele {operator} în timp real. Vezi orarele, plecările de la stații, traseele liniilor și planifică-ți călătoria.",
		routeStopsFmt:      "Stațiile liniei %s spre %s",
		stopLines:          "Liniile care opresc aici",
		towardFmt:          "spre %s",
		lineEnds:           "capăt de linie",
		stopAreaFmt:        "Cum ajungi în %s",
		destTitleFmt:       "Ce autobuz iau până în %s? Linii și timpi — {city}",
		destDescFmt:        "Liniile directe până în %s, {city}, din fiecare zonă: unde urci, unde cobori, cât durează și cât de des circulă.",
		destH1Fmt:          "Ce autobuz iau până în %s?",
		destLinesFmt:       "Liniile care opresc în %s: %s.",
		destFrom:           "Din alte zone",
		destFromFmt:        "Din %s",
		rideFmt:            "linia %s, de la stația %s până la %s, %s",
		rideEveryFmt:       "la ~%d min",
		rideSpanFmt:        "primul %s, ultimul %s",
		rideNoWeekdays:     "nu circulă în zilele lucrătoare",
		destNoRides:        "Nicio linie directă din alte zone; planifică drumul cu schimbare.",
		destStopsFmt:       "Stații în %s",
		destPlanFmt:        "Planifică drumul până în %s",
		destAll:            "Destinațiile din {city}",
		allLines:           "Liniile din {city}",
		lineItemFmt:        "Linia %s — %s",
		destsTitle:         "Destinații în {city} — Ce autobuz iau?",
		destsDesc:          "Cartierele și localitățile în care ajung liniile {operator}, cu liniile directe până în fiecare.",
		destsH1:            "Destinații în {city}",
		destsDistricts:     "Cartiere",
		destsTowns:         "Localități",
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
		homeTitle:          "{city} bus times — live timetables | Ce autobuz iau?",
		homeDesc:           "{city} bus timetables in real time: live departures from every stop, the timetable and route of every {operator} line, and a trip planner.",
		homeH1:             "{city} public transport in real time",
		homeBody:           "{operator} buses, trolleybuses and trams in real time. See timetables, stop departures, line routes and plan your trip.",
		routeStopsFmt:      "Line %s stops toward %s",
		stopLines:          "Lines stopping here",
		towardFmt:          "toward %s",
		lineEnds:           "last stop",
		stopAreaFmt:        "How to get to %s",
		destTitleFmt:       "Which bus to %s? Lines and times — {city} | Ce autobuz iau?",
		destDescFmt:        "Direct lines to %s, {city}, from every area: where to get on and off, how long it takes and how often they run.",
		destH1Fmt:          "Which bus goes to %s?",
		destLinesFmt:       "Lines stopping in %s: %s.",
		destFrom:           "From other areas",
		destFromFmt:        "From %s",
		rideFmt:            "line %s, from %s to %s, %s",
		rideEveryFmt:       "every ~%d min",
		rideSpanFmt:        "first %s, last %s",
		rideNoWeekdays:     "no weekday service",
		destNoRides:        "No direct line from other areas; plan a trip with a change.",
		destStopsFmt:       "Stops in %s",
		destPlanFmt:        "Plan a trip to %s",
		destAll:            "Destinations in {city}",
		allLines:           "Lines in {city}",
		lineItemFmt:        "Line %s — %s",
		destsTitle:         "Destinations in {city} — Ce autobuz iau?",
		destsDesc:          "The districts and towns {operator} lines reach, with the direct lines to each.",
		destsH1:            "Destinations in {city}",
		destsDistricts:     "Districts",
		destsTowns:         "Towns and villages",
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
	l.destTitleFmt = fill(l.destTitleFmt)
	l.destDescFmt = fill(l.destDescFmt)
	l.destAll = fill(l.destAll)
	l.allLines = fill(l.allLines)
	l.destsTitle = fill(l.destsTitle)
	l.destsDesc = fill(l.destsDesc)
	l.destsH1 = fill(l.destsH1)
	return l
}

type metaInjection struct {
	title       string
	description string
	canonical   string // absolute URL of canonical version (Romanian, no lang query)
	bodyHTML    string // raw HTML inserted inside #app for crawlers; Vue replaces on mount
	jsonLD      string // raw JSON for application/ld+json (already JSON-encoded, no HTML escape)
	// Shown by the app until its own data arrives.
	name string
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
		app := `<div id="app">`
		if m.name != "" {
			app = `<div id="app" data-name="` + html.EscapeString(m.name) + `">`
		}
		body = reAppMount.ReplaceAllLiteralString(body, app+`<div style="position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)">`+m.bodyHTML+`</div></div>`)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Cache-Control", web.Revalidate)
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

// A ?lang=en page is its own canonical.
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

func routePage(city *cities.City, c fiber.Ctx, routeID, direction int) error {
	routes, err := city.Routes(transit.RouteFilter{RouteID: &routeID})
	if err != nil {
		return c.Next()
	}
	if len(routes) == 0 {
		return missing(c)
	}
	shortName, longName := routes[0].RouteShortName, routes[0].RouteLongName

	l := ogTexts(city, c)
	routeURL := canonicalURL(city, lineCanonicalPath(city, routeID, direction))
	title := fmt.Sprintf(l.routeTitleFmt, shortName, directedName(longName, direction))
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
		name:        shortName,
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(fmt.Sprintf(l.routeH1Fmt, shortName)),
			html.EscapeString(fmt.Sprintf(l.routeBodyFmt, shortName, longName))) + routeStopsHTML(c, l, city, routeID, direction),
		jsonLD: jsonLD,
	})
}

// Same as directedRouteName in frontend/src/utils/time.ts.
func directedName(longName string, dir int) string {
	parts := strings.Split(longName, " - ")
	if dir == 1 {
		slices.Reverse(parts)
	}
	return strings.Join(parts, " → ")
}

func publisherOrg() map[string]any {
	return map[string]any{
		"@type": "Organization",
		"name":  "Ce autobuz iau?",
		"url":   web.SiteOrigin + "/",
	}
}

func stopPage(city *cities.City, c fiber.Ctx, stopID int) error {
	stops, err := city.Stops(transit.StopFilter{StopID: &stopID})
	if err != nil {
		return c.Next()
	}
	if len(stops) == 0 {
		return missing(c)
	}
	stopName, stopLat, stopLon := stops[0].StopName, stops[0].StopLat, stops[0].StopLon

	l := ogTexts(city, c)
	stopURL := canonicalURL(city, stationPath(city, stops[0].Station()))
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
		name:        stopName,
		bodyHTML: fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
			html.EscapeString(fmt.Sprintf(l.stopH1Fmt, stopName)),
			html.EscapeString(fmt.Sprintf(l.stopBodyFmt, stopName))) + stationLinesHTML(c, l, city, stopID),
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
			html.EscapeString(l.homeBody)) + allLinesHTML(c, l, city) +
			fmt.Sprintf(`<p><a href="%s">%s</a></p>`, pagePath(c, city, destinationsPath), html.EscapeString(l.destAll)),
		jsonLD: marshalJSONLD(map[string]any{
			"@context": "https://schema.org",
			"@type":    "WebSite",
			"name":     "Ce autobuz iau?",
			"url":      canonicalURL(city, "/"),
		}),
	})
}
