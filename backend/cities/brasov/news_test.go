package brasov

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news/newstest"
	"testing"
)

func TestNews(t *testing.T) {
	page := `<h2 class="titlu-pagina">Noutati</h2>` +
		`<div class="stire-interior"><a href="/noutati/linie-temporara-41b-pe-traseul-dintre-livada-po%c8%99tei-%c8%99i-cimitirul-municipal_862/" title="Linie temporară"><h2>Linie temporară, 41B pe traseul dintre Livada Poștei și Cimitirul Municipal</h2></a>` +
		`<span class="data">30.10.2026</span><p><p>La solicitarea Primăriei Municipiului Brașov...</p>` +
		`<a href="/noutati/linie-temporara-41b-pe-traseul-dintre-livada-po%c8%99tei-%c8%99i-cimitirul-municipal_862/" class="citeste-mai-mult">Citeste mai mult <span>&#xf178;</span></a></div>` +
		`<div class="stire-interior"><a href="/noutati/proiectul-pilot-al-liniei-56t-continua-inca-trei-luni_860/"><h2>Proiectul-pilot al liniei 56T continuă încă trei luni</h2></a><span class="data">26.10.2026</span></div>` +
		`<div class="navigare"><ul><li><a href="/noutati/p-1/">1</a></li></ul></div>`
	newstest.Expect(t, newstest.Parse(t, parseNews, page, "https://www.ratbv.ro/noutati/"),
		feeds.NewsItem{Title: "Linie temporară, 41B pe traseul dintre Livada Poștei și Cimitirul Municipal", URL: "https://www.ratbv.ro/noutati/linie-temporara-41b-pe-traseul-dintre-livada-po%c8%99tei-%c8%99i-cimitirul-municipal_862/", At: newstest.Day(2026, 10, 30)},
		feeds.NewsItem{Title: "Proiectul-pilot al liniei 56T continuă încă trei luni", URL: "https://www.ratbv.ro/noutati/proiectul-pilot-al-liniei-56t-continua-inca-trei-luni_860/", At: newstest.Day(2026, 10, 26)},
	)
}
