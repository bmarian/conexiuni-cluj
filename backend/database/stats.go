package database

import (
	"log"
	"sync"
	"time"
)

const statsFlushInterval = 2 * time.Minute

type statsBuffer struct {
	visitors map[string]struct{}
	counters map[string]map[string]int
}

func newStatsBuffer() *statsBuffer {
	return &statsBuffer{
		visitors: make(map[string]struct{}),
		counters: make(map[string]map[string]int),
	}
}

var (
	statsMu     sync.Mutex
	statsBuf    = newStatsBuffer()
	statsTZ     *time.Location
	statsTZOnce sync.Once
)

func statsLocation() *time.Location {
	statsTZOnce.Do(func() {
		loc, err := time.LoadLocation("Europe/Bucharest")
		if err != nil {
			loc = time.UTC
		}
		statsTZ = loc
	})
	return statsTZ
}

func todayKey() string {
	return time.Now().In(statsLocation()).Format("2006-01-02")
}

// RecordEvent buffers a single request observation in memory.
// Safe to call from hot paths — only a mutex + map writes.
func RecordEvent(clientHash, metric, key string) {
	if clientHash == "" && metric == "" {
		return
	}
	statsMu.Lock()
	if clientHash != "" {
		statsBuf.visitors[clientHash] = struct{}{}
	}
	if metric != "" {
		bucket, ok := statsBuf.counters[metric]
		if !ok {
			bucket = make(map[string]int)
			statsBuf.counters[metric] = bucket
		}
		bucket[key]++
	}
	statsMu.Unlock()
}

func swapBuffer() *statsBuffer {
	statsMu.Lock()
	prev := statsBuf
	statsBuf = newStatsBuffer()
	statsMu.Unlock()
	return prev
}

func mergeBuffer(buf *statsBuffer) {
	statsMu.Lock()
	for hash := range buf.visitors {
		statsBuf.visitors[hash] = struct{}{}
	}
	for metric, bucket := range buf.counters {
		current, ok := statsBuf.counters[metric]
		if !ok {
			current = make(map[string]int)
			statsBuf.counters[metric] = current
		}
		for key, count := range bucket {
			current[key] += count
		}
	}
	statsMu.Unlock()
}

func writeStatsBuffer(buf *statsBuffer) error {
	today := todayKey()

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	visitorUpsert, err := tx.Prepare(`
		INSERT INTO stats_visitors (client_hash, first_seen, last_seen)
		VALUES (?, ?, ?)
		ON CONFLICT(client_hash) DO UPDATE SET last_seen = excluded.last_seen
	`)
	if err != nil {
		return err
	}
	defer func() { _ = visitorUpsert.Close() }()

	visitorDaily, err := tx.Prepare(`
		INSERT OR IGNORE INTO stats_visitors_daily (date, client_hash) VALUES (?, ?)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = visitorDaily.Close() }()

	for hash := range buf.visitors {
		if _, err := visitorUpsert.Exec(hash, today, today); err != nil {
			return err
		}
		if _, err := visitorDaily.Exec(today, hash); err != nil {
			return err
		}
	}

	dailyUpsert, err := tx.Prepare(`
		INSERT INTO stats_daily (date, metric, key, count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(date, metric, key) DO UPDATE SET count = count + excluded.count
	`)
	if err != nil {
		return err
	}
	defer func() { _ = dailyUpsert.Close() }()

	for metric, bucket := range buf.counters {
		for key, count := range bucket {
			if _, err := dailyUpsert.Exec(today, metric, key, count); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func FlushStats() error {
	buf := swapBuffer()
	if len(buf.visitors) == 0 && len(buf.counters) == 0 {
		return nil
	}
	if err := writeStatsBuffer(buf); err != nil {
		mergeBuffer(buf)
		return err
	}
	return nil
}

func StartStatsFlusher() {
	go func() {
		ticker := time.NewTicker(statsFlushInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := FlushStats(); err != nil {
				log.Printf("stats flush failed: %v", err)
			}
		}
	}()
}

type VisitorTotals struct {
	Lifetime int `json:"lifetime"`
	Last7d   int `json:"last7d"`
	Today    int `json:"today"`
}

func GetVisitorTotals() (VisitorTotals, error) {
	var t VisitorTotals
	today := todayKey()
	weekAgo := time.Now().In(statsLocation()).AddDate(0, 0, -6).Format("2006-01-02")

	if err := DB.QueryRow(`SELECT COUNT(*) FROM stats_visitors`).Scan(&t.Lifetime); err != nil {
		return t, err
	}
	if err := DB.QueryRow(`SELECT COUNT(DISTINCT client_hash) FROM stats_visitors_daily WHERE date >= ?`, weekAgo).Scan(&t.Last7d); err != nil {
		return t, err
	}
	if err := DB.QueryRow(`SELECT COUNT(*) FROM stats_visitors_daily WHERE date = ?`, today).Scan(&t.Today); err != nil {
		return t, err
	}
	return t, nil
}

type DailyVisitorPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func GetDailyVisitors(days int) ([]DailyVisitorPoint, error) {
	if days <= 0 {
		days = 30
	}
	start := time.Now().In(statsLocation()).AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := DB.Query(`
		SELECT date, COUNT(*) FROM stats_visitors_daily
		WHERE date >= ?
		GROUP BY date
		ORDER BY date ASC
	`, start)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	byDate := make(map[string]int)
	for rows.Next() {
		var d string
		var c int
		if err := rows.Scan(&d, &c); err != nil {
			return nil, err
		}
		byDate[d] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]DailyVisitorPoint, 0, days)
	loc := statsLocation()
	startDate := time.Now().In(loc).AddDate(0, 0, -(days - 1))
	for i := 0; i < days; i++ {
		d := startDate.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, DailyVisitorPoint{Date: d, Count: byDate[d]})
	}
	return out, nil
}

type TopEntry struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

func GetTopMetric(metric string, limit int) ([]TopEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	extraWhere := ""
	if metric == "api_call" {
		extraWhere = " AND key NOT LIKE '/api/admin%'"
	}
	rows, err := DB.Query(`
		SELECT key, SUM(count) AS total
		FROM stats_daily
		WHERE metric = ?`+extraWhere+`
		GROUP BY key
		ORDER BY total DESC
		LIMIT ?
	`, metric, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]TopEntry, 0, limit)
	for rows.Next() {
		var e TopEntry
		if err := rows.Scan(&e.Key, &e.Count); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func GetMetricTotal(metric, key string) (int, error) {
	var total int
	err := DB.QueryRow(`
		SELECT COALESCE(SUM(count), 0)
		FROM stats_daily
		WHERE metric = ? AND key = ?
	`, metric, key).Scan(&total)
	return total, err
}
