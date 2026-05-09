package handlers

import (
	"conexiuni-cluj/database"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/net/html"
)

type NewsItem struct {
	URL   string `json:"url"`
	Date  string `json:"date"`
	Title string `json:"title"`
}

const ctpNewsPageURL = "https://www.ctpcj.ro/index.php/ro/despre-noi/stiri"

func GetNews(c fiber.Ctx, shelfLife time.Duration) error {
	if database.IsCacheValid("news") {
		if items, err := loadNewsFromDB(); err == nil && len(items) > 0 {
			c.Set("Cache-Control", "max-age=1800, stale-while-revalidate=3600")
			return c.JSON(items)
		}
	}

	fresh, err := fetchNewsFromCTP()
	if err != nil {
		if stale, dbErr := loadNewsFromDB(); dbErr == nil && len(stale) > 0 {
			c.Set("Cache-Control", "max-age=60")
			return c.JSON(stale)
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "news unavailable"})
	}

	_ = saveNewsToDB(fresh)
	_ = database.UpdateCache("news", shelfLife.Milliseconds())

	c.Set("Cache-Control", "max-age=1800, stale-while-revalidate=3600")
	return c.JSON(fresh)
}

func saveNewsToDB(items []NewsItem) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM news_cache`); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO news_cache (url, date, title) VALUES (?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, item := range items {
		if _, err := stmt.Exec(item.URL, item.Date, item.Title); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func loadNewsFromDB() ([]NewsItem, error) {
	rows, err := database.DB.Query(`SELECT url, date, title FROM news_cache`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []NewsItem
	for rows.Next() {
		var item NewsItem
		if err := rows.Scan(&item.URL, &item.Date, &item.Title); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func fetchNewsFromCTP() ([]NewsItem, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, ctpNewsPageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ConexiuniCluj/1.0)")
	req.Header.Set("Accept-Language", "ro,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	type rawItem struct {
		url   string
		title string
		date  string
	}
	var pending []rawItem
	var items []NewsItem

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var itemProp string
			for _, a := range n.Attr {
				if a.Key == "itemprop" {
					itemProp = a.Val
					break
				}
			}

			switch itemProp {
			case "url":
				u := newsNodeAttr(n, "href")
				if u == "" {
					u = newsNodeAttr(n, "content")
				}
				if u != "" {
					if strings.HasPrefix(u, "/") {
						u = "https://www.ctpcj.ro" + u
					}
					title := strings.TrimSpace(newsNodeText(n))
					pending = append(pending, rawItem{url: u, title: title})
				}
			case "dateCreated":
				d := newsNodeAttr(n, "datetime")
				if d == "" {
					d = newsNodeAttr(n, "content")
				}
				if d == "" {
					d = newsNodeText(n)
				}
				if d = strings.TrimSpace(d); d != "" && len(pending) > 0 {
					last := pending[len(pending)-1]
					pending = pending[:len(pending)-1]
					items = append(items, NewsItem{URL: last.url, Title: last.title, Date: d})
				}
			}
		}

		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	return items, nil
}

func newsNodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func newsNodeText(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return sb.String()
}
