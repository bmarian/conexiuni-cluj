package news

import (
	"conexiuni-cluj/sources"
	"net/url"
	"testing"
	"time"
)

var bucharest, _ = time.LoadLocation("Europe/Bucharest")

func parse(t *testing.T, parser Parser, page, base string) []sources.NewsItem {
	t.Helper()
	u, _ := url.Parse(base)
	items, err := parser([]byte(page), u, bucharest)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func expect(t *testing.T, got []sources.NewsItem, want ...sources.NewsItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d items %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i].Title != want[i].Title || got[i].URL != want[i].URL || !got[i].At.Equal(want[i].At) {
			t.Errorf("item %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, bucharest) }

func TestRSS(t *testing.T) {
	page := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0"><channel><title>Site</title>
<item><title>Linia 21  deviată &#8211; temporar</title><link>https://stpt.ro/linia-21/?utm_source=rss&amp;utm_medium=rss</link><pubDate>Mon, 28 Sep 2026 15:07:26 +0000</pubDate></item>
<item><title>Automate noi &amp;amp; abonamente</title><link>/index.php/ro/stiri/1880</link><pubDate>Fri, 9 Oct 2026 08:00:00 +0300</pubDate></item>
<item><title>No date</title><link>https://stpt.ro/x/</link></item>
</channel></rss>`
	expect(t, parse(t, RSS, page, "https://www.ctpcj.ro/index.php/ro/despre-noi/stiri?format=feed&type=rss"),
		sources.NewsItem{Title: "Linia 21 deviată – temporar", URL: "https://stpt.ro/linia-21/", At: time.Date(2026, 9, 28, 15, 7, 26, 0, time.UTC)},
		sources.NewsItem{Title: "Automate noi & abonamente", URL: "https://www.ctpcj.ro/index.php/ro/stiri/1880", At: time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC)},
	)
}

func TestSCTP(t *testing.T) {
	page := `<div class="tab-content align-left" id="tab-ratp">
<div class="stire-ratp halfsized"><h3>Modificări în circulație</h3><span class="data">15.10.2026</span>
<p>Având în vedere</p><a href="/stiri/modificari.html" class="readmore">citește tot</a></div>
<div class="stire-ratp halfsized"><h3>Fără dată</h3><a href="/stiri/x.html" class="readmore">citește tot</a></div>
<div class="stire-ratp halfsized"><h3>Posturi vacante de şoferi autobuz</h3><span class="data">19.10.2026</span>
<p> COMPANIA DE TRANSPORT PUBLIC Iaşi organizează </p><a href="/stiri/posturi-vacante-de-oferi-autobuz.html" class="readmore">citește tot</a></div>
<div class="stire-ratp halfsized"><h3>Post vacant de mecanic utilaj</h3><span class="data">29.05.2025</span>
<a href="/stiri/post-vacant-de-mecanic-utilaj.html" class="readmore">citește tot</a></div>
<div class="stire-ratp halfsized"><h3>Selecție în vederea ocupării posturilor vacante de şoferi autobuz</h3><span class="data">30.07.2020</span>
<a href="/stiri/selectie.html" class="readmore">citește tot</a></div>
<div class="stire-ratp halfsized"><h3>Angajare electronist telecomunicații</h3><span class="data">13.10.2017</span>
<a href="/stiri/angajare.html" class="readmore">citește tot</a></div>
<div class="stire-ratp halfsized"><h3>Curse speciale pentru angajaţii Spitalului modular COVID de la Leţcani</h3><span class="data">13.10.2021</span>
<a href="/stiri/curse-speciale.html" class="readmore">citește tot</a></div>
</div>`
	expect(t, parse(t, SCTP, page, "https://www.sctpiasi.ro/ro/stiri"),
		sources.NewsItem{Title: "Modificări în circulație", URL: "https://www.sctpiasi.ro/stiri/modificari.html", At: day(2026, 10, 15)},
		sources.NewsItem{Title: "Curse speciale pentru angajaţii Spitalului modular COVID de la Leţcani", URL: "https://www.sctpiasi.ro/stiri/curse-speciale.html", At: day(2021, 10, 13)},
	)
}

func TestCTBus(t *testing.T) {
	page := `<div id="hrefComunicate">
<div id="20261013-1" class="hashTarget"></div>
<div class="recente-stirea clearfix"><div class="recente-meta"><div class="recente-tip"><i class="fa fa-newspaper-o"></i></div></div>
<h3 class="recente-titlu"><a href="#20261013-1" class="du-te-la">` + "️" + `TRASEELE LINIILOR 48 ȘI 101, MODIFICATE TEMPORAR</a></h3></div>
<div id="20261015-0" class="hashTarget"></div>
<div class="recente-stirea clearfix"><div class="recente-meta"><div class="recente-tip"><i class="fa fa-user-plus"></i></div></div>
<h3 class="recente-titlu"><a href="#20261015-0" class="du-te-la">Angajări - MECANIC</a></h3></div>
</div>`
	expect(t, parse(t, CTBus, page, "https://www.ctbus.ro/comunicate"),
		sources.NewsItem{Title: "TRASEELE LINIILOR 48 ȘI 101, MODIFICATE TEMPORAR", URL: "https://www.ctbus.ro/comunicate#20261013-1", At: day(2026, 10, 13)},
	)
}

func TestTursib(t *testing.T) {
	page := `<div class="card"><div class="card-header time"><h5><time datetime="2026-10-13" title="13 Oct 2026"><strong>13</strong></time></h5></div>
<div class="info col"><article><header class="entry-header"><h3 class="entry-title"><a rel="bookmark" href="/news/sisteme-indisponibile">SISTEME TEMPORAR INDISPONIBILE</a></h3></header>
<a class="more-link" href="/news/sisteme-indisponibile">Citește mai mult</a></article></div></div>
<div class="card"><div class="info col"><article><h3 class="entry-title"><a href="/news/fara-data">Fără dată</a></h3></article></div></div>`
	expect(t, parse(t, Tursib, page, "https://tursib.ro/news"),
		sources.NewsItem{Title: "SISTEME TEMPORAR INDISPONIBILE", URL: "https://tursib.ro/news/sisteme-indisponibile", At: day(2026, 10, 13)},
	)
}
