package live

type Bus struct {
	Label   string  `json:"label"`
	RouteID int     `json:"route_id"`
	TripID  string  `json:"trip_id"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

func (eng *Engine) Buses() []Bus {
	table := eng.currentLiveTable()
	if table == nil {
		return nil
	}
	var out []Bus
	for tripID, vehicles := range table.vehiclesByTrip {
		for _, v := range vehicles {
			if v.Label != "" {
				out = append(out, Bus{Label: v.Label, RouteID: v.RouteID, TripID: tripID, Lat: v.Latitude, Lon: v.Longitude})
			}
		}
	}
	return out
}
