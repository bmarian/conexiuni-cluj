package cities

import (
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"math"
)

func (c *City) HideStops(ids []int, hidden bool) error {
	if err := c.SetStopsHidden(ids, hidden); err != nil {
		return err
	}
	if err := c.offered.Refresh(c.Data); err != nil {
		return err
	}
	c.Live.RefreshTrips()
	c.Network.Invalidate()
	c.Network.Warm()
	c.Planner.Forget()
	c.running().Go(c.Planner.Warm)
	return nil
}

func (c *City) StopInstead(stopID int) (models.Stop, bool) {
	stops, err := c.Stops(transit.StopFilter{})
	if err != nil {
		return models.Stop{}, false
	}
	var from, nearest models.Stop
	for _, s := range stops {
		if s.StopID == stopID {
			from = s
		}
	}
	best := math.Inf(1)
	for _, s := range stops {
		if d := geo.Haversine(from.StopLat, from.StopLon, s.StopLat, s.StopLon); d < best && c.offered.StopHasBuses(s.StopID) {
			best, nearest = d, s
		}
	}
	return nearest, !math.IsInf(best, 1)
}
