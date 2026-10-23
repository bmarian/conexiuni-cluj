package destinations

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"fmt"
	"math"
	"sort"
)

func (r Ride) expected() float64 {
	if r.EveryMin == 0 {
		return math.Inf(1)
	}
	return float64(r.EveryMin)/2 + float64(r.Minutes[0]+r.Minutes[1])/2
}

type service struct {
	times []float64
	every float64
}

func serviceOn(t *models.Timetable, dayType string, dir int) (service, bool) {
	if t == nil {
		return service{}, false
	}
	if times := transit.Departures(t, dayType, dir); len(times) > 0 {
		return service{times: times, every: medianGap(times)}, true
	}
	day := t.Weekdays
	switch dayType {
	case calendar.Saturday:
		day = t.Saturday
	case calendar.Sunday:
		day = t.Sunday
	}
	f := day.InFrequency
	if dir == 1 {
		f = day.OutFrequency
	}
	if f == nil {
		return service{}, false
	}
	start, okStart := transit.ParseDeparture(f.Start)
	end, okEnd := transit.ParseDeparture(f.End)
	if !okStart || !okEnd || end < start {
		return service{}, false
	}
	var times []float64
	for m := start; m < end; m += 60 {
		times = append(times, float64(m))
	}
	return service{times: append(times, float64(end)), every: float64(f.MaxMinutes)}, true
}

func medianGap(times []float64) float64 {
	var gaps []float64
	for i := 1; i < len(times); i++ {
		if g := times[i] - times[i-1]; g > 0 {
			gaps = append(gaps, g)
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	sort.Float64s(gaps)
	return gaps[len(gaps)/2]
}

func (x *Index) ride(p *pattern, i, j int) (Ride, bool) {
	for _, day := range calendar.ScheduleDays {
		svc, ok := serviceOn(p.route.Timetable, day, p.dir)
		if !ok {
			continue
		}
		reach := func(at float64, k int) float64 {
			hours := p.reach[day]
			h := int(at/60) % 24
			if h >= len(hours) || k >= len(hours[h]) {
				return 0
			}
			return hours[h][k]
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, at := range svc.times {
			secs := reach(at, j) - reach(at, i)
			lo, hi = math.Min(lo, secs), math.Max(hi, secs)
		}
		first, last := svc.times[0], svc.times[len(svc.times)-1]
		return Ride{
			RouteID: p.route.RouteID, Direction: p.dir, Board: p.stops[i], Alight: p.stops[j],
			Minutes:  [2]int{rideMinutes(lo), rideMinutes(hi)},
			EveryMin: int(math.Round(svc.every)),
			First:    clock(first + reach(first, i)/60),
			Last:     clock(last + reach(last, i)/60),
			DayType:  day,
		}, true
	}
	return Ride{}, false
}

func rideMinutes(secs float64) int { return max(1, int(math.Round(secs/60))) }

func clock(minutes float64) string {
	m := int(math.Round(minutes)) % (24 * 60)
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}
