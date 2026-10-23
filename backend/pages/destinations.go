package pages

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/cities"
	"conexiuni-cluj/destinations"
	"fmt"
	"html"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func pagePath(c fiber.Ctx, city *cities.City, path string) string {
	return html.EscapeString(selfURL(c, "/"+city.Slug+path))
}

func cityIndex(city *cities.City) *destinations.Index {
	x, err := city.Destinations.Index()
	if err != nil {
		return nil
	}
	return x
}

func lineNames(x *destinations.Index, ids []int) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if r := x.Route(id); r != nil {
			names = append(names, r.RouteShortName)
		}
	}
	return strings.Join(names, ", ")
}

func routeStopsHTML(c fiber.Ctx, l ogLocale, city *cities.City, routeID, dir int) string {
	x := cityIndex(city)
	if x == nil || x.Route(routeID) == nil {
		return ""
	}
	stops := x.RouteStops(routeID, dir)
	if len(stops) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `<h2>%s</h2><ol>`, html.EscapeString(fmt.Sprintf(l.routeStopsFmt, x.Route(routeID).RouteShortName, x.Toward(routeID, dir))))
	for _, s := range stops {
		fmt.Fprintf(&sb, `<li><a href="%s">%s</a></li>`, pagePath(c, city, "/stop/"+strconv.Itoa(s.StopID)), html.EscapeString(s.StopName))
	}
	sb.WriteString(`</ol>`)
	if other := 1 - dir; len(x.RouteStops(routeID, other)) > 0 {
		fmt.Fprintf(&sb, `<p><a href="%s">%s</a></p>`, pagePath(c, city, fmt.Sprintf("/route/%d/%d", routeID, other)),
			html.EscapeString(fmt.Sprintf(l.routeOtherFmt, x.Toward(routeID, other))))
	}
	return sb.String()
}

func allLinesHTML(c fiber.Ctx, l ogLocale, city *cities.City) string {
	x := cityIndex(city)
	if x == nil {
		return ""
	}
	offered := city.Availability()
	var sb strings.Builder
	for _, r := range x.Lines() {
		if offered.IsReady() && !offered.RouteHasTimetable(r.RouteShortName) {
			continue
		}
		fmt.Fprintf(&sb, `<li><a href="%s">%s</a></li>`, pagePath(c, city, fmt.Sprintf("/route/%d/0", r.RouteID)),
			html.EscapeString(fmt.Sprintf(l.lineItemFmt, r.RouteShortName, r.RouteLongName)))
	}
	if sb.Len() == 0 {
		return ""
	}
	return fmt.Sprintf(`<h2>%s</h2><ul>%s</ul>`, html.EscapeString(l.allLines), sb.String())
}

func stationLinesHTML(c fiber.Ctx, l ogLocale, city *cities.City, stopID int) string {
	x := cityIndex(city)
	if x == nil {
		return ""
	}
	groups := x.StationLines(stopID)
	var sb strings.Builder
	if len(groups) > 0 {
		fmt.Fprintf(&sb, `<h2>%s</h2><ul>`, html.EscapeString(l.stopLines))
		for _, g := range groups {
			links := make([]string, 0, len(g.Lines))
			for _, line := range g.Lines {
				links = append(links, fmt.Sprintf(`<a href="%s">%s</a>`, pagePath(c, city, fmt.Sprintf("/route/%d/%d", line.RouteID, line.Direction)),
					html.EscapeString(x.Route(line.RouteID).RouteShortName)))
			}
			end := l.lineEnds
			if g.Toward != "" {
				end = fmt.Sprintf(l.towardFmt, g.Toward)
			}
			fmt.Fprintf(&sb, `<li>%s %s</li>`, strings.Join(links, ", "), html.EscapeString(end))
		}
		sb.WriteString(`</ul>`)
	}
	if area := x.AreaOf(stopID); area != nil {
		fmt.Fprintf(&sb, `<p><a href="%s">%s</a></p>`, pagePath(c, city, "/to/"+area.Slug), html.EscapeString(fmt.Sprintf(l.stopAreaFmt, area.Name)))
	}
	return sb.String()
}

func rideText(l ogLocale, x *destinations.Index, r destinations.Ride) string {
	minutes := fmt.Sprintf("%d–%d min", r.Minutes[0], r.Minutes[1])
	if r.Minutes[0] == r.Minutes[1] {
		minutes = fmt.Sprintf("%d min", r.Minutes[0])
	}
	parts := []string{fmt.Sprintf(l.rideFmt, x.Route(r.RouteID).RouteShortName, x.Stop(r.Board).StopName, x.Stop(r.Alight).StopName, minutes)}
	if r.EveryMin > 0 {
		parts = append(parts, fmt.Sprintf(l.rideEveryFmt, r.EveryMin))
	}
	parts = append(parts, fmt.Sprintf(l.rideSpanFmt, r.First, r.Last))
	if r.DayType != calendar.Weekday {
		parts = append(parts, l.rideNoWeekdays)
	}
	return strings.Join(parts, ", ")
}

func destinationPage(city *cities.City, c fiber.Ctx) error {
	x, ready, err := city.Destinations.Areas()
	if err != nil || !ready {
		return c.Next()
	}
	dest := x.Destination(c.Params("slug"))
	if dest == nil {
		// The app still loads, to suggest similar names.
		if indexHTMLContent == "" {
			return c.Next()
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusNotFound).SendString(indexHTMLContent)
	}
	l := ogTexts(city, c)
	pageURL := canonicalURL(city, "/to/"+dest.Slug)
	title := fmt.Sprintf(l.destTitleFmt, dest.Name)
	desc := fmt.Sprintf(l.destDescFmt, dest.Name)

	var sb strings.Builder
	fmt.Fprintf(&sb, `<h1>%s</h1><p>%s</p>`, html.EscapeString(fmt.Sprintf(l.destH1Fmt, dest.Name)),
		html.EscapeString(fmt.Sprintf(l.destLinesFmt, dest.Name, lineNames(x, dest.RouteIDs))))
	fmt.Fprintf(&sb, `<h2>%s</h2>`, html.EscapeString(l.destFrom))
	if len(dest.From) == 0 {
		fmt.Fprintf(&sb, `<p>%s</p>`, html.EscapeString(l.destNoRides))
	}
	for _, o := range dest.From {
		fmt.Fprintf(&sb, `<h3><a href="%s">%s</a></h3><ul>`, pagePath(c, city, "/to/"+o.Slug), html.EscapeString(fmt.Sprintf(l.destFromFmt, o.Name)))
		for _, r := range o.Rides {
			fmt.Fprintf(&sb, `<li>%s</li>`, html.EscapeString(rideText(l, x, r)))
		}
		sb.WriteString(`</ul>`)
	}
	fmt.Fprintf(&sb, `<h2>%s</h2><ul>`, html.EscapeString(fmt.Sprintf(l.destStopsFmt, dest.Name)))
	for _, id := range dest.StopIDs {
		var lines []int
		for _, g := range x.StationLines(id) {
			for _, line := range g.Lines {
				if !slices.Contains(lines, line.RouteID) {
					lines = append(lines, line.RouteID)
				}
			}
		}
		fmt.Fprintf(&sb, `<li><a href="%s">%s</a>: %s</li>`, pagePath(c, city, "/stop/"+strconv.Itoa(id)),
			html.EscapeString(x.Stop(id).StopName), html.EscapeString(lineNames(x, lines)))
	}
	sb.WriteString(`</ul>`)
	plan := url.Values{"name": {dest.Name}, "lat": {strconv.FormatFloat(dest.Lat, 'f', 6, 64)}, "lon": {strconv.FormatFloat(dest.Lon, 'f', 6, 64)}}
	fmt.Fprintf(&sb, `<p><a href="%s">%s</a></p>`, pagePath(c, city, "/plan?"+plan.Encode()), html.EscapeString(fmt.Sprintf(l.destPlanFmt, dest.Name)))
	fmt.Fprintf(&sb, `<p><a href="%s">%s</a></p>`, pagePath(c, city, "/destinations"), html.EscapeString(l.destAll))

	return injectMeta(c, l, metaInjection{
		title:       title,
		description: desc,
		canonical:   pageURL,
		bodyHTML:    sb.String(),
		jsonLD: marshalJSONLD(map[string]any{
			"@context":    "https://schema.org",
			"@type":       "WebPage",
			"name":        title,
			"description": desc,
			"url":         selfURL(c, pageURL),
			"inLanguage":  l.lang,
			"publisher":   publisherOrg(),
			"about": map[string]any{
				"@type": "Place",
				"name":  dest.Name,
				"geo": map[string]any{
					"@type":     "GeoCoordinates",
					"latitude":  dest.Lat,
					"longitude": dest.Lon,
				},
				"containedInPlace": map[string]any{"@type": "City", "name": city.Name},
			},
		}),
	})
}

func destinationsPage(city *cities.City, c fiber.Ctx) error {
	x, ready, err := city.Destinations.Areas()
	if err != nil || !ready {
		return c.Next()
	}
	l := ogTexts(city, c)
	pageURL := canonicalURL(city, "/destinations")
	var sb strings.Builder
	fmt.Fprintf(&sb, `<h1>%s</h1><p>%s</p>`, html.EscapeString(l.destsH1), html.EscapeString(l.destsDesc))
	for _, districts := range []bool{true, false} {
		heading := l.destsTowns
		if districts {
			heading = l.destsDistricts
		}
		fmt.Fprintf(&sb, `<h2>%s</h2><ul>`, html.EscapeString(heading))
		for _, d := range x.Destinations() {
			if d.District == districts {
				fmt.Fprintf(&sb, `<li><a href="%s">%s</a></li>`, pagePath(c, city, "/to/"+d.Slug), html.EscapeString(fmt.Sprintf(l.destH1Fmt, d.Name)))
			}
		}
		sb.WriteString(`</ul>`)
	}
	return injectMeta(c, l, metaInjection{
		title:       l.destsTitle,
		description: l.destsDesc,
		canonical:   pageURL,
		bodyHTML:    sb.String(),
		jsonLD: marshalJSONLD(map[string]any{
			"@context":    "https://schema.org",
			"@type":       "CollectionPage",
			"name":        l.destsTitle,
			"description": l.destsDesc,
			"url":         selfURL(c, pageURL),
			"inLanguage":  l.lang,
			"publisher":   publisherOrg(),
		}),
	})
}
