package pages

import (
	"conexiuni-cluj/cities"
	"conexiuni-cluj/web"
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

const SourcesPath = "/surse"

func Sources(registry *cities.Registry) fiber.Handler {
	return func(c fiber.Ctx) error {
		l := detectLocale(c)
		list := slices.Clone(registry.Enabled())
		order := collate.New(language.Romanian)
		slices.SortFunc(list, func(a, b *cities.City) int { return order.CompareString(a.Name, b.Name) })
		var sb strings.Builder
		fmt.Fprintf(&sb, `<h1>%s</h1><p>%s</p><ul>`, html.EscapeString(l.sourcesH1), html.EscapeString(l.sourcesDesc))
		for _, city := range list {
			fmt.Fprintf(&sb, `<li>%s: <a href="%s">%s</a>`, html.EscapeString(city.Name), html.EscapeString(city.OperatorURL), html.EscapeString(city.Operator))
			for _, credit := range city.Credits() {
				fmt.Fprintf(&sb, ` · <a href="%s">%s</a>`, html.EscapeString(credit.URL), html.EscapeString(credit.Name))
			}
			sb.WriteString(`</li>`)
		}
		fmt.Fprintf(&sb, `</ul><p>%s</p>`, html.EscapeString(l.sourcesMap))
		pageURL := web.SiteOrigin + SourcesPath
		return injectMeta(c, l, metaInjection{
			title:       l.sourcesTitle,
			description: l.sourcesDesc,
			canonical:   pageURL,
			bodyHTML:    sb.String(),
			jsonLD: marshalJSONLD(map[string]any{
				"@context":    "https://schema.org",
				"@type":       "WebPage",
				"name":        l.sourcesTitle,
				"description": l.sourcesDesc,
				"url":         selfURL(c, pageURL),
				"inLanguage":  l.lang,
				"publisher":   publisherOrg(),
			}),
		})
	}
}
