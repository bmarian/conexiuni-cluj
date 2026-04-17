package ctp_cj

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v3/client"
	"golang.org/x/time/rate"
)

type DayOfTheWeek string

const (
	DayLV DayOfTheWeek = "lv"
	DayS  DayOfTheWeek = "s"
	DayD  DayOfTheWeek = "d"
)

type Client struct {
	BaseURL string
	client  *client.Client
	limiter *rate.Limiter
}

var ErrNotFound = errors.New("timetable not found")

func (c *Client) doRequest(routeShortName string, day DayOfTheWeek) ([]byte, error) {
	url := fmt.Sprintf("%s/orar_%s_%s.csv", c.BaseURL, routeShortName, day)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == 404 {
		return nil, ErrNotFound
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode())
	}
	return resp.Body(), nil
}

// FetchTimetable best-effort loads available day files and skips failed days.
func (c *Client) FetchTimetable(routeShortName string) (weekdays, saturday, sunday *ParsedTimetable, err error) {
	if err = c.limiter.Wait(context.Background()); err != nil {
		return nil, nil, nil, fmt.Errorf("ctpcj: rate limiter: %w", err)
	}

	for _, pair := range []struct {
		day DayOfTheWeek
		dst **ParsedTimetable
	}{
		{DayLV, &weekdays},
		{DayS, &saturday},
		{DayD, &sunday},
	} {
		data, reqErr := c.doRequest(routeShortName, pair.day)
		if errors.Is(reqErr, ErrNotFound) {
			continue
		}
		if reqErr != nil {
			log.Printf("ctpcj: skip %s/%s: %v", routeShortName, pair.day, reqErr)
			continue
		}
		parsed, parseErr := ParseTimetableCSV(data)
		if parseErr != nil {
			log.Printf("ctpcj: skip %s/%s: parse error: %v", routeShortName, pair.day, parseErr)
			continue
		}
		*pair.dst = parsed
	}
	return weekdays, saturday, sunday, nil
}

func NewClient(baseUrl string, rateLimit time.Duration) *Client {
	c := client.New()
	c.SetTimeout(30 * time.Second)

	return &Client{
		BaseURL: baseUrl,
		client:  c,
		limiter: rate.NewLimiter(rate.Every(rateLimit), 1),
	}
}
