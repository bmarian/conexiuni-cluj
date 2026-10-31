package router

import "math"

// The best way that swaps one of best's rides for other lines, unless waiting for its last listed bus takes less.
func (n *Network) secondWay(best, walk Option, q Query, find func(skip map[int32]bool) []Option) (Option, bool) {
	limit := int64(math.MaxInt64)
	// With fewer listed, walking beat its later buses.
	if len(best.Runs) >= q.Runs {
		wait := best
		wait.Runs = best.Runs[len(best.Runs)-1:]
		_, limit = n.score(wait, q)
	}
	var out Option
	ok := false
	for _, skip := range best.ridePatterns {
		for _, o := range n.worthTaking(find(skip), walk, q) {
			_, effort := n.score(o, q)
			ticketPaysOff := o.Tickets <= best.Tickets || when(o, q) < when(best, q)
			if effort <= limit && ticketPaysOff && !adds(best, o) {
				out, ok, limit = o, true, effort-1
			}
		}
	}
	return out, ok
}
