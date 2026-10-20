package iasi

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news/newstest"
	"testing"
)

func TestNews(t *testing.T) {
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
	newstest.Expect(t, newstest.Parse(t, parseNews, page, "https://www.sctpiasi.ro/ro/stiri"),
		feeds.NewsItem{Title: "Modificări în circulație", URL: "https://www.sctpiasi.ro/stiri/modificari.html", At: newstest.Day(2026, 10, 15)},
		feeds.NewsItem{Title: "Curse speciale pentru angajaţii Spitalului modular COVID de la Leţcani", URL: "https://www.sctpiasi.ro/stiri/curse-speciale.html", At: newstest.Day(2021, 10, 13)},
	)
}
