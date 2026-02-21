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

type dailyQuota struct {
	mutex   sync.Mutex
	count   int
	limit   int
	resetAt time.Time
	loc     *time.Location
}

func (quota *dailyQuota) check() error {
	quota.mutex.Lock()
	defer quota.mutex.Unlock()

	now := time.Now().In(quota.loc)
	if now.After(quota.resetAt) {
		quota.count = 0
		y, m, d := now.Date()
		quota.resetAt = time.Date(y, m, d+1, 0, 0, 0, 0, quota.loc)
	}

	if quota.count >= quota.limit {
		return errors.New("tranzy: daily quota exceeded for endpoint")
	}

	quota.count++
	return nil
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

func (c *Client) quota(endpoint string) *dailyQuota {
	if endpoint == "/vehicles" {
		return c.vehiclesQuota
	}
	return c.defaultQuota
}

func (c *Client) DoRequest(endpoint string, params map[string]string) ([]byte, error) {
	if err := c.quota(endpoint).check(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, endpoint)
	}

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

func NewClient(baseUrl string, apiKey string, agencyId string) *Client {
	c := client.New()
	c.SetTimeout(30 * time.Second)

	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		log.Printf("Warning: could not load Europe/Bucharest timezone, falling back to UTC: %v", err)
		loc = time.UTC
	}

	return &Client{
		BaseURL:       baseUrl,
		APIKey:        apiKey,
		AgencyId:      agencyId,
		client:        c,
		limiter:       rate.NewLimiter(rate.Limit(5), 5),
		vehiclesQuota: &dailyQuota{limit: 4500, loc: loc},
		defaultQuota:  &dailyQuota{limit: 500, loc: loc},
	}
}
