package planner

import (
	"math"

	"conexiuni-cluj/router"
)

type planEnd struct {
	StopID int `json:"stop_id"`
	Meters int `json:"meters"`
}

// Ends planned to or from their nearest stop instead.
type planBeyond struct {
	From *planEnd `json:"from,omitempty"`
	To   *planEnd `json:"to,omitempty"`
}

func beyondReach(n *router.Network, q *router.Query) *planBeyond {
	var b planBeyond
	nearest := func(p *router.Point) *planEnd {
		if n.Reaches(*p) {
			return nil
		}
		stop, meters := n.Nearest(*p)
		*p = router.Point{Lat: stop.Lat, Lon: stop.Lon}
		return &planEnd{StopID: stop.ID, Meters: int(math.Round(meters))}
	}
	b.From, b.To = nearest(&q.From), nearest(&q.To)
	if b.From == nil && b.To == nil {
		return nil
	}
	return &b
}
