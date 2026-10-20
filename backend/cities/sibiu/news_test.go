package sibiu

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news/newstest"
	"testing"
)

func TestNews(t *testing.T) {
	page := `<div class="card"><div class="card-header time"><h5><time datetime="2026-10-13" title="13 Oct 2026"><strong>13</strong></time></h5></div>
<div class="info col"><article><header class="entry-header"><h3 class="entry-title"><a rel="bookmark" href="/news/sisteme-indisponibile">SISTEME TEMPORAR INDISPONIBILE</a></h3></header>
<a class="more-link" href="/news/sisteme-indisponibile">Citește mai mult</a></article></div></div>
<div class="card"><div class="info col"><article><h3 class="entry-title"><a href="/news/fara-data">Fără dată</a></h3></article></div></div>`
	newstest.Expect(t, newstest.Parse(t, parseNews, page, "https://tursib.ro/news"),
		feeds.NewsItem{Title: "SISTEME TEMPORAR INDISPONIBILE", URL: "https://tursib.ro/news/sisteme-indisponibile", At: newstest.Day(2026, 10, 13)},
	)
}
