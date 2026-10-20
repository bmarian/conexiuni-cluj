package news_test

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news"
	"conexiuni-cluj/feeds/news/newstest"
	"testing"
	"time"
)

func TestRSS(t *testing.T) {
	page := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0"><channel><title>Site</title>
<item><title>Linia 21  deviată &#8211; temporar</title><link>https://stpt.ro/linia-21/?utm_source=rss&amp;utm_medium=rss</link><pubDate>Mon, 28 Sep 2026 15:07:26 +0000</pubDate></item>
<item><title>Automate noi &amp;amp; abonamente</title><link>/index.php/ro/stiri/1880</link><pubDate>Fri, 9 Oct 2026 08:00:00 +0300</pubDate></item>
<item><title>No date</title><link>https://stpt.ro/x/</link></item>
</channel></rss>`
	newstest.Expect(t, newstest.Parse(t, news.RSS, page, "https://www.ctpcj.ro/index.php/ro/despre-noi/stiri?format=feed&type=rss"),
		feeds.NewsItem{Title: "Linia 21 deviată – temporar", URL: "https://stpt.ro/linia-21/", At: time.Date(2026, 9, 28, 15, 7, 26, 0, time.UTC)},
		feeds.NewsItem{Title: "Automate noi & abonamente", URL: "https://www.ctpcj.ro/index.php/ro/stiri/1880", At: time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC)},
	)
}
