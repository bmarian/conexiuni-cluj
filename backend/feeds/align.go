package feeds

import (
	"conexiuni-cluj/models"
	"log"
)

func FirstStopNames(snap *Snapshot, routeID int) [2]string {
	var first [2]models.TripStop
	var found [2]bool
	patterns := map[string]int{}
	for _, t := range snap.Trips {
		if t.RouteID == routeID {
			patterns[t.TripID] = int(t.DirectionID)
		}
	}
	for _, ts := range snap.TripStops {
		dir, ok := patterns[ts.TripID]
		if !ok || dir < 0 || dir > 1 {
			continue
		}
		if !found[dir] || ts.StopSequence < first[dir].StopSequence {
			first[dir], found[dir] = ts, true
		}
	}
	names := map[int]string{}
	for _, s := range snap.Stops {
		names[s.StopID] = s.StopName
	}
	return [2]string{names[first[0].StopID], names[first[1].StopID]}
}

// AlignDirections swaps in and out when the termini match the Snapshot's directions the other way round.
func AlignDirections(t *models.Timetable, first [2]string, source string) {
	if (t.InStopName == "" && t.OutStopName == "") || first[0] == "" || first[1] == "" {
		return
	}
	keep := stopMatchScore(t.InStopName, first[0]) + stopMatchScore(t.OutStopName, first[1])
	swap := stopMatchScore(t.InStopName, first[1]) + stopMatchScore(t.OutStopName, first[0])
	if swap <= keep {
		return
	}
	log.Printf("%s: timetable %s: swapping directions (in=%q out=%q, first stops %q / %q)", source, t.RouteShortName, t.InStopName, t.OutStopName, first[0], first[1])
	t.InStopName, t.OutStopName = t.OutStopName, t.InStopName
	for _, d := range []*models.DaySchedule{&t.Weekdays, &t.Saturday, &t.Sunday} {
		for i := range d.Entries {
			d.Entries[i].DepartureIn, d.Entries[i].DepartureOut = d.Entries[i].DepartureOut, d.Entries[i].DepartureIn
		}
		d.InFrequency, d.OutFrequency = d.OutFrequency, d.InFrequency
	}
}
