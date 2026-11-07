package ramnicuvalcea

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news/newstest"
	"testing"
)

func TestNews(t *testing.T) {
	page := `<div class="archive-timeline"><article id="post-2526" class="post-2526 post type-post status-publish format-standard hentry entry" data-date="15 Sep 2026">` +
		`<header class="entry-header"><h3 class="entry-title"><a rel="bookmark" href="/comunicate/tutorial-abonamente-gratuite-elevi/">Tutorial obținere online abonamente gratuite de transport pentru elevii din Râmnicu Vâlcea</a></h3></header>` +
		`<div class="entry-content"><div class="read-more-wrapper"><a class="more-link " href="/comunicate/tutorial-abonamente-gratuite-elevi/">Citește mai mult</a></div></div></article>` +
		`<article id="post-2513" class="post-2513 post type-post status-publish format-standard hentry entry" data-date=" 6 Iul 2026">` +
		`<header class="entry-header"><h3 class="entry-title"><a rel="bookmark" href="/comunicate/anunt-vanzare-2026-07-06/">Anunț vânzare</a></h3></header></article></div>`
	newstest.Expect(t, newstest.Parse(t, parseNews, page, "https://eta-bus.ro/comunicate"),
		feeds.NewsItem{Title: "Tutorial obținere online abonamente gratuite de transport pentru elevii din Râmnicu Vâlcea", URL: "https://eta-bus.ro/comunicate/tutorial-abonamente-gratuite-elevi/", At: newstest.Day(2026, 9, 15)},
		feeds.NewsItem{Title: "Anunț vânzare", URL: "https://eta-bus.ro/comunicate/anunt-vanzare-2026-07-06/", At: newstest.Day(2026, 7, 6)},
	)
}
