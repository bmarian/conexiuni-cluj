package tranzy

import (
	"conexiuni-cluj/models"
	"encoding/json"
	"fmt"
)

func (c *Client) GetRoutes() ([]models.Route, error) {
	data, err := c.DoRequest("/routes")
	if err != nil {
		return nil, err
	}

	var routes []models.Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal routes: %w", err)
	}

	return routes, nil
}
