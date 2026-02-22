package ctp_cj

import (
	"context"
	"errors"
	"fmt"
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
		data, err := c.doRequest(routeShortName, pair.day)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s/%s: %w", routeShortName, pair.day, err)
		}
		parsed, err := ParseTimetableCSV(data)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse %s/%s: %w", routeShortName, pair.day, err)
		}
		*pair.dst = parsed
	}
	return
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
