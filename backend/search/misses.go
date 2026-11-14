package search

import (
	"strings"

	"conexiuni-cluj/planner"
	"conexiuni-cluj/router"
)

const (
	missNothing = "nothing"
	missFar     = "far"
)

// Keys "reason|query".
func (s *Service) noteMiss(q string, a answer) {
	reason := ""
	switch {
	case a.Notice != nil:
		reason = a.Notice.Kind
	case a.Best == bestFar:
		reason = missFar
	case a.empty():
		reason = missNothing
	default:
		return
	}
	if text := strings.Join(router.Words(q), " "); text != "" {
		s.store.RecordEvent(planner.PlaceMissMetric, reason+"|"+text)
	}
}
