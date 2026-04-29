package ors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type QuotaPersister interface {
	Load(name string) (count int, resetAt time.Time, err error)
	Save(name string, count int, resetAt time.Time) error
}

type dailyQuota struct {
	mutex     sync.Mutex
	name      string
	count     int
	limit     int
	resetAt   time.Time
	loc       *time.Location
	persister QuotaPersister
}

func (q *dailyQuota) check() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.rolloverLocked()

	if q.count >= q.limit {
		return errors.New("ors: daily quota exceeded")
	}

	q.count++
	q.persistLocked()
	return nil
}

func (q *dailyQuota) rolloverLocked() {
	now := time.Now().In(q.loc)
	if now.After(q.resetAt) {
		q.count = 0
		y, m, d := now.Date()
		q.resetAt = time.Date(y, m, d+1, 0, 0, 0, 0, q.loc)
		q.persistLocked()
	}
}

func (q *dailyQuota) persistLocked() {
	if q.persister == nil {
		return
	}
	if err := q.persister.Save(q.name, q.count, q.resetAt); err != nil {
		log.Printf("ors: failed to persist quota %q: %v", q.name, err)
	}
}

func (q *dailyQuota) remaining() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.rolloverLocked()
	r := q.limit - q.count
	if r < 0 {
		r = 0
	}
	return r
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	limiter    *rate.Limiter
	quota      *dailyQuota
}

var allowedProfiles = map[string]bool{
	"foot-walking":     true,
	"foot-hiking":      true,
	"cycling-regular":  true,
	"cycling-road":     true,
	"cycling-electric": true,
	"driving-car":      true,
	"wheelchair":       true,
}

func NewClient(baseURL, apiKey string, dailyLimit, minuteLimit int, persister QuotaPersister) *Client {
	q := &dailyQuota{
		name:      "ors-directions",
		limit:     dailyLimit,
		loc:       time.UTC,
		persister: persister,
	}
	if persister != nil {
		count, resetAt, err := persister.Load("ors-directions")
		if err != nil {
			log.Printf("ors: failed to load persisted quota: %v", err)
		} else if !resetAt.IsZero() {
			q.count = count
			q.resetAt = resetAt
		}
	}
	q.mutex.Lock()
	q.rolloverLocked()
	q.mutex.Unlock()

	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		// burst=5 handles concurrent users without blowing the per-minute average
		limiter: rate.NewLimiter(rate.Limit(float64(minuteLimit)/60.0), 5),
		quota:   q,
	}
}

func (c *Client) QuotaRemaining() int {
	return c.quota.remaining()
}

func (c *Client) GetDirections(profile string, fromLat, fromLng, toLat, toLng float64) ([]byte, error) {
	if !allowedProfiles[profile] {
		return nil, fmt.Errorf("ors: unsupported profile %q", profile)
	}

	if err := c.quota.check(); err != nil {
		return nil, fmt.Errorf("%w: profile=%s", err, profile)
	}

	if err := c.limiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("ors: rate limiter: %w", err)
	}

	// ORS expects [longitude, latitude] order.
	payload, err := json.Marshal(map[string]any{
		"coordinates": [][]float64{{fromLng, fromLat}, {toLng, toLat}},
	})
	if err != nil {
		return nil, fmt.Errorf("ors: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v2/directions/%s", c.baseURL, profile)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ors: build request profile=%s: %w", profile, err)
	}
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ors: POST directions/%s: %w", profile, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ors: read response profile=%s: %w", profile, err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ors: POST directions/%s status=%d body=%q",
			profile, resp.StatusCode, truncate(string(body), 500))
	}

	return body, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
