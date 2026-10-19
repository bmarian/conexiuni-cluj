package handlers

import (
	"conexiuni-cluj/sources"
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

func (city *City) StartNews(r *runner) {
	if city.News == nil {
		return
	}
	r.Go(func() {
		for {
			// Refetched in development too, unlike the network: a news page costs no quota.
			if !r.Sleep(time.Until(city.fetchedAt(ingestNews).Add(city.News.Cadence()))) {
				return
			}
			items, err := city.News.News(city.Location)
			if err == nil {
				err = city.storeNews(items)
			}
			if err != nil {
				city.log("news: %v", err)
				if !r.Sleep(city.News.Cadence()) {
					return
				}
				continue
			}
			city.markFetched(ingestNews, time.Now())
		}
	})
}

func (city *City) storeNews(items []sources.NewsItem) error {
	return batchReplace(city.DB,
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

func (city *City) recentNews() ([]Update, error) {
	return queryRows(city.DB,
		`SELECT url, title, published_at FROM news WHERE published_at >= ? ORDER BY published_at DESC`,
		[]any{time.Now().Add(-updateRetention).UnixMilli()},
		func(rows *sql.Rows) (Update, error) {
			u := Update{Kind: "news", News: &NewsItem{}}
			return u, rows.Scan(&u.News.URL, &u.News.Title, &u.At)
		})
}

func (city *City) GetUpdates(c fiber.Ctx) error {
	updates, err := city.recentNews()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	changes, err := city.routeChangesOf(routeNamesQuery(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	for i := range changes {
		updates = append(updates, Update{Kind: "change", At: changes[i].DetectedAt, Change: &changes[i]})
	}
	sort.SliceStable(updates, func(i, j int) bool { return updates[i].At > updates[j].At })
	c.Set("Cache-Control", revalidateCacheControl)
	return c.JSON(updates)
}
