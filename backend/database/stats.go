package database

import (
	"log"
	"strings"
	"sync"
	"time"
)

const statsFlushInterval = 2 * time.Minute

type statsVisit struct {
	hash   string
	client string
}

type statsBuffer struct {
	visits   map[statsVisit]struct{}
	counters map[string]map[string]int
}

func newStatsBuffer() *statsBuffer {
	return &statsBuffer{
		visits:   make(map[statsVisit]struct{}),
		counters: make(map[string]map[string]int),
	}
}

var (
	// Client hashes are deleted after this many days, like the access logs.
	visitorRetentionDays int
	statsMu              sync.Mutex
	statsBuf             = newStatsBuffer()
	statsTZ              *time.Location
	statsTZOnce          sync.Once
)

func StatsLocation() *time.Location {
	statsTZOnce.Do(func() {
		loc, err := time.LoadLocation("Europe/Bucharest")
		if err != nil {
			loc = time.UTC
		}
		statsTZ = loc
	})
	return statsTZ
}

func StatsDate(t time.Time) string {
	return t.In(StatsLocation()).Format("2006-01-02")
}

func StatsDaysAgo(days int) string {
	return time.Now().In(StatsLocation()).AddDate(0, 0, -days).Format("2006-01-02")
}

// RecordVisit notes one client of a kind (web, android/1.4, ...) for today's counts.
func RecordVisit(clientHash, client string) {
	if clientHash == "" || client == "" {
		return
	}
	statsMu.Lock()
	statsBuf.visits[statsVisit{hash: clientHash, client: client}] = struct{}{}
	statsMu.Unlock()
}

func RecordEvent(metric, key string) {
	RecordCount(metric, key, 1)
}

func RecordCount(metric, key string, count int) {
	if metric == "" || count <= 0 {
		return
	}
	statsMu.Lock()
	incrementCounterLocked(metric, strings.Clone(key), count)
	statsMu.Unlock()
}

func RecordEndpointTiming(key string, duration time.Duration) {
	ms := int(duration / time.Millisecond)
	if duration > 0 && ms == 0 {
		ms = 1
	}
	statsMu.Lock()
	incrementCounterLocked("endpoint_call", key, 1)
	incrementCounterLocked("endpoint_ms", key, ms)
	statsMu.Unlock()
}

func RecordTranzyQuotaUsage(name string) {
	RecordEvent("tranzy_quota", name)
}

func incrementCounterLocked(metric, key string, count int) {
	bumpCounter(statsBuf, metric, key, count)
}

func bumpCounter(buf *statsBuffer, metric, key string, count int) {
	bucket, ok := buf.counters[metric]
	if !ok {
		bucket = make(map[string]int)
		buf.counters[metric] = bucket
	}
	bucket[key] += count
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
	for v := range buf.visits {
		statsBuf.visits[v] = struct{}{}
	}
	for metric, bucket := range buf.counters {
		for key, count := range bucket {
			incrementCounterLocked(metric, key, count)
		}
	}
	statsMu.Unlock()
}

func writeStatsBuffer(buf *statsBuffer) error {
	today := StatsDate(time.Now())
	cutoff := StatsDaysAgo(visitorRetentionDays - 1)

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	seenBefore, err := tx.Prepare(`SELECT EXISTS(SELECT 1 FROM stats_visitors_daily WHERE client_hash = ? AND date < ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = seenBefore.Close() }()
	visitorDaily, err := tx.Prepare(`INSERT OR IGNORE INTO stats_visitors_daily (date, client_hash) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = visitorDaily.Close() }()
	clientDaily, err := tx.Prepare(`INSERT OR IGNORE INTO stats_clients_daily (date, client, client_hash) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = clientDaily.Close() }()

	hashes := make(map[string]struct{}, len(buf.visits))
	for v := range buf.visits {
		hashes[v.hash] = struct{}{}
		res, err := clientDaily.Exec(today, v.client, v.hash)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			bumpCounter(buf, "client_visitors", v.client, 1)
		}
	}
	for hash := range hashes {
		var seen bool
		if err := seenBefore.QueryRow(hash, today).Scan(&seen); err != nil {
			return err
		}
		res, err := visitorDaily.Exec(today, hash)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		bumpCounter(buf, "visitors", "", 1)
		if !seen {
			bumpCounter(buf, "new_visitors", "", 1)
		}
	}

	if _, err := tx.Exec(`DELETE FROM stats_visitors_daily WHERE date < ?`, cutoff); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM stats_clients_daily WHERE date < ?`, cutoff); err != nil {
		return err
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
	if len(buf.visits) == 0 && len(buf.counters) == 0 {
		return nil
	}
	if err := writeStatsBuffer(buf); err != nil {
		mergeBuffer(buf)
		return err
	}
	return nil
}

func StartStatsFlusher(retentionDays int) {
	visitorRetentionDays = retentionDays
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

type MetricRow struct {
	Date  string
	Key   string
	Count int
}

// MetricRows returns a metric's daily counts from `since` (inclusive), unflushed counts included.
func MetricRows(metric, since string) ([]MetricRow, error) {
	rows, err := DB.Query(`SELECT date, key, count FROM stats_daily WHERE metric = ? AND date >= ?`, metric, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]MetricRow, 0)
	for rows.Next() {
		var r MetricRow
		if err := rows.Scan(&r.Date, &r.Key, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	today := StatsDate(time.Now())
	statsMu.Lock()
	for key, count := range statsBuf.counters[metric] {
		out = append(out, MetricRow{Date: today, Key: key, Count: count})
	}
	statsMu.Unlock()
	return out, nil
}

func MetricTotal(metric, key string) (int, error) {
	var total int
	err := DB.QueryRow(`SELECT COALESCE(SUM(count), 0) FROM stats_daily WHERE metric = ? AND key = ?`, metric, key).Scan(&total)
	return total, err
}
