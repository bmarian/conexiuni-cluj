package tranzy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3/client"
	"golang.org/x/time/rate"
)

// QuotaPersister Tranzy does not reset the daily quota on our restart, so we persist the count.
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

func (quota *dailyQuota) check() error {
	quota.mutex.Lock()
	defer quota.mutex.Unlock()

	quota.rolloverLocked()

	if quota.count >= quota.limit {
		return errors.New("tranzy: daily quota exceeded for endpoint")
	}

	quota.count++
	quota.persistLocked()
	return nil
}

func (quota *dailyQuota) rolloverLocked() {
	now := time.Now().In(quota.loc)
	if now.After(quota.resetAt) {
		quota.count = 0
		y, m, d := now.Date()
		quota.resetAt = time.Date(y, m, d+1, 0, 0, 0, 0, quota.loc)
		quota.persistLocked()
	}
}

func (quota *dailyQuota) persistLocked() {
	if quota.persister == nil {
		return
	}
	if err := quota.persister.Save(quota.name, quota.count, quota.resetAt); err != nil {
		log.Printf("tranzy: failed to persist quota %q: %v", quota.name, err)
	}
}

func (quota *dailyQuota) remaining() int {
	quota.mutex.Lock()
	defer quota.mutex.Unlock()
	quota.rolloverLocked()
	r := quota.limit - quota.count
	if r < 0 {
		r = 0
	}
	return r
}

type Client struct {
	BaseURL       string
	APIKey        string
	AgencyId      string
	client        *client.Client
	limiter       *rate.Limiter
	vehiclesQuota *dailyQuota
	defaultQuota  *dailyQuota
}

func (c *Client) VehiclesQuotaRemaining() int {
	return c.vehiclesQuota.remaining()
}

func (c *Client) VehiclesQuotaLimit() int {
	return c.vehiclesQuota.limit
}

func (c *Client) Location() *time.Location {
	return c.vehiclesQuota.loc
}

func (c *Client) quota(endpoint string) *dailyQuota {
	if endpoint == "/vehicles" {
		return c.vehiclesQuota
	}
	return c.defaultQuota
}

func (c *Client) DoRequest(endpoint string, params map[string]string) ([]byte, error) {
	q := c.quota(endpoint)
	if err := q.check(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, endpoint)
	}
	log.Printf("tranzy: request %s (quota %d/%d used)", endpoint, q.limit-q.remaining(), q.limit)

	if err := c.limiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("tranzy: rate limiter: %w", err)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, endpoint)
	resp, err := c.client.Get(url, client.Config{
		Header: map[string]string{
			"X-API-KEY":   c.APIKey,
			"X-AGENCY-ID": c.AgencyId,
		},
		Param: params,
	})

	if err != nil {
		return nil, err
	}

	return resp.Body(), nil
}

func newQuota(name string, limit int, loc *time.Location, persister QuotaPersister) *dailyQuota {
	q := &dailyQuota{name: name, limit: limit, loc: loc, persister: persister}
	if persister != nil {
		count, resetAt, err := persister.Load(name)
		if err != nil {
			log.Printf("tranzy: failed to load persisted quota %q: %v", name, err)
		} else if !resetAt.IsZero() {
			q.count = count
			q.resetAt = resetAt
			log.Printf("tranzy: restored quota %q: %d/%d used, resets at %s",
				name, count, limit, resetAt.In(loc).Format(time.RFC3339))
		}
	}
	q.mutex.Lock()
	q.rolloverLocked()
	q.mutex.Unlock()
	return q
}

func NewClient(baseUrl string, apiKey string, agencyId string, rateLimit time.Duration, vehiclesDailyQuota int, defaultDailyQuota int, persister QuotaPersister) *Client {
	c := client.New()
	c.SetTimeout(30 * time.Second)

	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		log.Printf("Warning: could not load Europe/Bucharest timezone, falling back to UTC: %v", err)
		loc = time.UTC
	}

	burst := int(time.Second / rateLimit)

	return &Client{
		BaseURL:       baseUrl,
		APIKey:        apiKey,
		AgencyId:      agencyId,
		client:        c,
		limiter:       rate.NewLimiter(rate.Every(rateLimit), burst),
		vehiclesQuota: newQuota("vehicles", vehiclesDailyQuota, loc, persister),
		defaultQuota:  newQuota("default", defaultDailyQuota, loc, persister),
	}
}
