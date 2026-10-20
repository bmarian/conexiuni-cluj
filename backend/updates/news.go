package updates

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/web"
	"database/sql"
	"sort"
	"time"

	"github.com/gofiber/fiber/v3"
)

const ingestNews = "news"

type NewsItem struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Update struct {
	Kind   string       `json:"kind"`
	At     int64        `json:"at"`
	News   *NewsItem    `json:"news,omitempty"`
	Change *RouteChange `json:"change,omitempty"`
}

// StartNews keeps the operator's news, read from source at its own pace.
func (s *Service) StartNews(r *jobs.Runner, source feeds.News) {
	if source == nil {
		return
	}
	r.Go(func() {
		for {
			// Refetched in development too, unlike the network: a news page costs no quota.
			if !r.Sleep(time.Until(s.store.FetchedAt(ingestNews).Add(source.Cadence()))) {
				return
			}
			items, err := source.News(s.loc)
			if err == nil {
				err = s.storeNews(items)
			}
			if err != nil {
				s.store.Logf("news: %v", err)
				if !r.Sleep(source.Cadence()) {
					return
				}
				continue
			}
			s.store.MarkFetched(ingestNews, time.Now())
		}
	})
}

func (s *Service) storeNews(items []feeds.NewsItem) error {
	return database.BatchReplace(s.store.DB,
		func(tx *sql.Tx) error {
			_, err := tx.Exec(`DELETE FROM news`)
			return err
		},
		`INSERT OR REPLACE INTO news (url, title, published_at) VALUES (?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, it := range items {
				if _, err := stmt.Exec(it.URL, it.Title, it.At.UnixMilli()); err != nil {
					return err
				}
			}
			return nil
		})
}

func (s *Service) recentNews() ([]Update, error) {
	return database.QueryRows(s.store.DB,
		`SELECT url, title, published_at FROM news WHERE published_at >= ? ORDER BY published_at DESC`,
		[]any{time.Now().Add(-updateRetention).UnixMilli()},
		func(rows *sql.Rows) (Update, error) {
			u := Update{Kind: "news", News: &NewsItem{}}
			return u, rows.Scan(&u.News.URL, &u.News.Title, &u.At)
		})
}

func (s *Service) updatesHandler(c fiber.Ctx) error {
	updates, err := s.recentNews()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	changes, err := s.routeChangesOf(routeNamesQuery(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	for i := range changes {
		updates = append(updates, Update{Kind: "change", At: changes[i].DetectedAt, Change: &changes[i]})
	}
	sort.SliceStable(updates, func(i, j int) bool { return updates[i].At > updates[j].At })
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(updates)
}
