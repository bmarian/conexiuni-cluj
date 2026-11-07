package albaiulia

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/news/newstest"
	"testing"
)

func TestNews(t *testing.T) {
	page := `<div class='stiri_col1'><div class='stiri_col1_title'>Comunicate de presa</div>` +
		`<div class='stire'><a href='presa.php?id=73'><div class='stire_data'>30-04-2026</div><div class='stire_titlu' style=''>Programul autobuzelor 1 Mai 2026</div>` +
		`<img src='images/comunicat_de_presa_thumb.jpg' class='stire_img'/><div class='stire_descriere'>Programul autobuzelor 1 Mai 2026 [...]</div></a></div>` +
		`<div class='stire'><a href='presa.php?id=67'><div class='stire_data'>06-10-2025</div><div class='stire_titlu' style=''>Reluare program normal</div></a></div></div>`
	newstest.Expect(t, newstest.Parse(t, parseNews, page, "https://www.stpalba.ro/presa.php"),
		feeds.NewsItem{Title: "Programul autobuzelor 1 Mai 2026", URL: "https://www.stpalba.ro/presa.php?id=73", At: newstest.Day(2026, 4, 30)},
		feeds.NewsItem{Title: "Reluare program normal", URL: "https://www.stpalba.ro/presa.php?id=67", At: newstest.Day(2025, 10, 6)},
	)
}
