package ctp_cj

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3/client"
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
}

var ErrNotFound = errors.New("timetable not found")

func (c *Client) DoRequest(routeShortName string, day DayOfTheWeek) ([]byte, error) {
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
	for _, pair := range []struct {
		day DayOfTheWeek
		dst **ParsedTimetable
	}{
		{DayLV, &weekdays},
		{DayS, &saturday},
		{DayD, &sunday},
	} {
		data, err := c.DoRequest(routeShortName, pair.day)
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
