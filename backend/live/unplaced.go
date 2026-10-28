package live

import (
	"conexiuni-cluj/models"
	"strconv"
	"strings"
)

const (
	UnplacedMetric = "bus_unplaced"
	SeenMetric     = "bus_seen"
)

// Keys "reason|route|bus", per poll; reason "trip" is a feed trip not running today, "route" a bus off its line.
func (eng *Engine) recordUnplaced(reason string, routeID int, v models.Vehicle) {
	bus := v.Label
	if bus == "" {
		bus = strconv.Itoa(v.ID)
	}
	eng.RecordEvent(UnplacedMetric, reason+"|"+strconv.Itoa(routeID)+"|"+strings.ReplaceAll(bus, "|", " "))
}
