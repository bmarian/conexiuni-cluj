package constanta

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news/newstest"
	"testing"
)

func TestNews(t *testing.T) {
	page := `<div id="hrefComunicate">
<div id="20261013-1" class="hashTarget"></div>
<div class="recente-stirea clearfix"><div class="recente-meta"><div class="recente-tip"><i class="fa fa-newspaper-o"></i></div></div>
<h3 class="recente-titlu"><a href="#20261013-1" class="du-te-la">` + "️" + `TRASEELE LINIILOR 48 ȘI 101, MODIFICATE TEMPORAR</a></h3></div>
<div id="20261015-0" class="hashTarget"></div>
<div class="recente-stirea clearfix"><div class="recente-meta"><div class="recente-tip"><i class="fa fa-user-plus"></i></div></div>
<h3 class="recente-titlu"><a href="#20261015-0" class="du-te-la">Angajări - MECANIC</a></h3></div>
</div>`
	newstest.Expect(t, newstest.Parse(t, parseNews, page, "https://www.ctbus.ro/comunicate"),
		feeds.NewsItem{Title: "TRASEELE LINIILOR 48 ȘI 101, MODIFICATE TEMPORAR", URL: "https://www.ctbus.ro/comunicate#20261013-1", At: newstest.Day(2026, 10, 13)},
	)
}
