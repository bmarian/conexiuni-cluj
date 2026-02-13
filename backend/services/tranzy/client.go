package tranzy

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3/client"
)

type Client struct {
	BaseURL  string
	APIKey   string
	AgencyId string
	client   *client.Client
}

func (c *Client) DoRequest(endpoint string, params map[string]string) ([]byte, error) {
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

	return &Client{
		BaseURL:  baseUrl,
		APIKey:   apiKey,
		AgencyId: agencyId,
		client:   c,
	}
}
