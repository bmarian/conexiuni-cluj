package updates

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/web"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	updateRetention = 30 * 24 * time.Hour

	SourceTimetable = "timetable"
	SourceStops     = "stops"

	Later        = "later"
	Earlier      = "earlier"
	TripsAdded   = "trips_added"
	TripsRemoved = "trips_removed"
	StopsAdded   = "stops_added"
	StopsRemoved = "stops_removed"

	// A departure that moved by more than this is treated as one trip removed and another added.
	maxDepartureShiftMinutes = 15
	unmatchedDepartureCost   = 8

	maxRouteChangesPerRequest = 200
	MaxFilterNames            = 100
)

type TimeShift struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type RouteChangeItem struct {
	Kind      string      `json:"kind"`
	Direction string      `json:"direction"`
	Toward    string      `json:"toward,omitempty"`
	Days      []string    `json:"days,omitempty"`
	Since     string      `json:"since,omitempty"`
	All       bool        `json:"all,omitempty"`
	Times     []string    `json:"times,omitempty"`
	Shifts    []TimeShift `json:"shifts,omitempty"`
	Stops     []string    `json:"stops,omitempty"`
}

type RouteChange struct {
	ID             int64             `json:"id"`
	RouteShortName string            `json:"route_short_name"`
	RouteID        int               `json:"route_id"`
	RouteColor     string            `json:"route_color"`
	RouteColorful  string            `json:"route_colorful"`
	Source         string            `json:"source"`
	DetectedAt     int64             `json:"detected_at"`
	Changes        []RouteChangeItem `json:"changes"`
}

func (s *Service) TrackTimetable(t *models.Timetable) error {
	previous, _ := s.data.Timetable(t.RouteShortName)
	if err := s.data.StoreTimetable(t); err != nil {
		return err
	}
	if previous != nil {
		s.Record(t.RouteShortName, SourceTimetable, diffTimetables(previous, t))
	}
	return nil
}

func (s *Service) TrackStopTimes(routeShortName string, stopTimes []models.StopTime) error {
	previous, err := s.data.StopTimesForTrips(transit.TripIDs(stopTimes))
	if err != nil {
		s.store.Logf("route changes: could not read previous stops for line %s: %v", routeShortName, err)
	}
	if err := s.data.StoreStopTimes(stopTimes); err != nil {
		return err
	}
	s.Record(routeShortName, SourceStops, s.diffRouteStops(previous, stopTimes))
	return nil
}

func diffTimetables(previous, current *models.Timetable) []RouteChangeItem {
	// Direction alignment can flip between fetches, so match columns by terminus names.
	swapped := previous.InStopName != previous.OutStopName &&
		previous.InStopName == current.OutStopName &&
		previous.OutStopName == current.InStopName

	days := []struct {
		key               string
		previous, current models.DaySchedule
	}{
		{"weekdays", previous.Weekdays, current.Weekdays},
		{"saturday", previous.Saturday, current.Saturday},
		{"sunday", previous.Sunday, current.Sunday},
	}

	var items []RouteChangeItem
	for _, day := range days {
		// A day that failed to download is zero-valued and says nothing about the schedule.
		if !dayFetched(day.previous) || !dayFetched(day.current) {
			continue
		}
		for _, direction := range []string{"0", "1"} {
			inColumn := direction == "0"
			if hasFrequency(day.previous, inColumn != swapped) || hasFrequency(day.current, inColumn) {
				continue
			}
			before := departureMinutes(day.previous.Entries, inColumn != swapped)
			after := departureMinutes(day.current.Entries, inColumn)
			if len(before) == 0 && len(after) == 0 {
				continue
			}
			if sameDeparturesWithinDay(before, after) {
				continue
			}
			toward := current.OutStopName
			if direction == "1" {
				toward = current.InStopName
			}
			base := RouteChangeItem{
				Direction: direction,
				Toward:    toward,
				Days:      []string{day.key},
				Since:     strings.TrimSpace(day.current.ServiceStart),
			}
			items = append(items, departureChanges(before, after, base)...)
		}
	}
	return mergeChangeDays(items)
}

func departureChanges(before, after []int, base RouteChangeItem) []RouteChangeItem {
	shifts, removed, added := alignDepartures(before, after)

	var later, earlier []TimeShift
	for _, s := range shifts {
		shift := TimeShift{From: formatDepartureMinutes(s[0]), To: formatDepartureMinutes(s[1])}
		if s[1] > s[0] {
			later = append(later, shift)
		} else {
			earlier = append(earlier, shift)
		}
	}

	var items []RouteChangeItem
	add := func(kind string, fill func(*RouteChangeItem)) {
		item := base
		item.Kind = kind
		fill(&item)
		items = append(items, item)
	}
	if len(later) > 0 {
		add(Later, func(i *RouteChangeItem) { i.Shifts = later })
	}
	if len(earlier) > 0 {
		add(Earlier, func(i *RouteChangeItem) { i.Shifts = earlier })
	}
	if len(removed) > 0 {
		add(TripsRemoved, func(i *RouteChangeItem) {
			i.Times = formatDepartureList(removed)
			i.All = len(after) == 0
		})
	}
	if len(added) > 0 {
		add(TripsAdded, func(i *RouteChangeItem) {
			i.Times = formatDepartureList(added)
			i.All = len(before) == 0
		})
	}
	return items
}

func dayFetched(d models.DaySchedule) bool {
	return d.ServiceStart != "" || d.ServiceName != "" || len(d.Entries) > 0 || d.InFrequency != nil || d.OutFrequency != nil
}

func hasFrequency(d models.DaySchedule, inColumn bool) bool {
	if inColumn {
		return d.InFrequency != nil
	}
	return d.OutFrequency != nil
}

// alignDepartures pairs old and new departures in order, preferring small shifts over add/remove pairs.
func alignDepartures(before, after []int) (shifts [][2]int, removed, added []int) {
	n, m := len(before), len(after)
	cost := make([][]int, n+1)
	for i := range cost {
		cost[i] = make([]int, m+1)
		cost[i][0] = i * unmatchedDepartureCost
	}
	for j := 1; j <= m; j++ {
		cost[0][j] = j * unmatchedDepartureCost
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			best := min(cost[i-1][j], cost[i][j-1]) + unmatchedDepartureCost
			if d := absInt(after[j-1] - before[i-1]); d <= maxDepartureShiftMinutes {
				best = min(best, cost[i-1][j-1]+d)
			}
			cost[i][j] = best
		}
	}

	for i, j := n, m; i > 0 || j > 0; {
		if i > 0 && j > 0 {
			d := absInt(after[j-1] - before[i-1])
			if d <= maxDepartureShiftMinutes && cost[i][j] == cost[i-1][j-1]+d {
				if d != 0 {
					shifts = append(shifts, [2]int{before[i-1], after[j-1]})
				}
				i, j = i-1, j-1
				continue
			}
		}
		if i > 0 && cost[i][j] == cost[i-1][j]+unmatchedDepartureCost {
			removed = append(removed, before[i-1])
			i--
			continue
		}
		added = append(added, after[j-1])
		j--
	}
	reverseSlice(shifts)
	reverseSlice(removed)
	reverseSlice(added)
	return shifts, removed, added
}

func departureMinutes(entries []models.TimetableEntry, inColumn bool) []int {
	out := make([]int, 0, len(entries))
	for _, e := range entries {
		raw := e.DepartureOut
		if inColumn {
			raw = e.DepartureIn
		}
		if m, ok := transit.ParseDeparture(raw); ok {
			out = append(out, m)
		}
	}
	sort.Ints(out)
	return out
}

// 35:38 and 11:38 are the same departure, written past midnight or not.
func sameDeparturesWithinDay(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	withinDay := func(xs []int) []int {
		out := make([]int, len(xs))
		for i, x := range xs {
			out[i] = x % 1440
		}
		sort.Ints(out)
		return out
	}
	wa, wb := withinDay(a), withinDay(b)
	for i := range wa {
		if wa[i] != wb[i] {
			return false
		}
	}
	return true
}

func formatDepartureMinutes(m int) string {
	return fmt.Sprintf("%02d:%02d", (m/60)%24, m%60)
}

func formatDepartureList(minutes []int) []string {
	out := make([]string, len(minutes))
	for i, m := range minutes {
		out[i] = formatDepartureMinutes(m)
	}
	return out
}

// Saturday and Sunday timetables are often identical, so the same change is listed once for both.
func mergeChangeDays(items []RouteChangeItem) []RouteChangeItem {
	var merged []RouteChangeItem
	index := make(map[string]int)
	for _, item := range items {
		probe := item
		probe.Days, probe.Since = nil, ""
		key, _ := json.Marshal(probe)
		if at, ok := index[string(key)]; ok {
			merged[at].Days = append(merged[at].Days, item.Days...)
			continue
		}
		index[string(key)] = len(merged)
		merged = append(merged, item)
	}
	return merged
}

func (s *Service) diffRouteStops(previous, current []models.StopTime) []RouteChangeItem {
	before := stopsByTrip(previous)
	after := stopsByTrip(current)

	tripIDs := make([]string, 0, len(after))
	for id := range after {
		tripIDs = append(tripIDs, id)
	}
	sort.Strings(tripIDs)

	var items []RouteChangeItem
	for _, tripID := range tripIDs {
		old, cur := before[tripID], after[tripID]
		direction, ok := models.PatternDirection(tripID)
		if !ok || len(old) == 0 || len(cur) == 0 {
			continue
		}
		removed, added, common := diffStopSequences(old, cur)
		if len(removed) == 0 && len(added) == 0 {
			continue
		}
		if common*2 < min(len(old), len(cur)) {
			s.store.Logf("route changes: ignoring stop diff for %s, only %d of %d/%d stops in common", tripID, common, len(old), len(cur))
			continue
		}
		base := RouteChangeItem{
			Direction: strconv.Itoa(direction),
			Toward:    strings.TrimSpace(cur[len(cur)-1].StopHeadsign),
		}
		if len(removed) > 0 {
			item := base
			item.Kind = StopsRemoved
			item.Stops = removed
			items = append(items, item)
		}
		if len(added) > 0 {
			item := base
			item.Kind = StopsAdded
			item.Stops = added
			items = append(items, item)
		}
	}
	return items
}

func stopsByTrip(stopTimes []models.StopTime) map[string][]models.StopTime {
	out := make(map[string][]models.StopTime)
	for _, st := range stopTimes {
		out[st.TripID] = append(out[st.TripID], st)
	}
	for _, stops := range out {
		sort.Slice(stops, func(i, j int) bool { return stops[i].StopSequence < stops[j].StopSequence })
	}
	return out
}

// diffStopSequences returns names of stops dropped from and added to the route, ignoring reordered ones.
func diffStopSequences(before, after []models.StopTime) (removed, added []string, common int) {
	n, m := len(before), len(after)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if before[i].StopID == after[j].StopID {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	var gone, fresh []models.StopTime
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case before[i].StopID == after[j].StopID:
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			gone = append(gone, before[i])
			i++
		default:
			fresh = append(fresh, after[j])
			j++
		}
	}
	gone = append(gone, before[i:]...)
	fresh = append(fresh, after[j:]...)

	goneIDs := make(map[int]bool, len(gone))
	for _, st := range gone {
		goneIDs[st.StopID] = true
	}
	freshIDs := make(map[int]bool, len(fresh))
	for _, st := range fresh {
		freshIDs[st.StopID] = true
	}
	for _, st := range gone {
		if !freshIDs[st.StopID] {
			removed = append(removed, strings.TrimSpace(st.StopHeadsign))
		}
	}
	for _, st := range fresh {
		if !goneIDs[st.StopID] {
			added = append(added, strings.TrimSpace(st.StopHeadsign))
		}
	}
	return removed, added, lcs[0][0]
}

func (s *Service) Record(routeShortName, source string, items []RouteChangeItem) {
	if len(items) == 0 {
		return
	}
	payload, err := json.Marshal(items)
	if err != nil {
		s.store.Logf("route changes: could not encode changes for line %s: %v", routeShortName, err)
		return
	}
	sum := sha1.Sum(append([]byte(source+"|"), payload...))
	signature := hex.EncodeToString(sum[:])
	now := time.Now()
	cutoff := now.Add(-updateRetention).UnixMilli()

	tx, err := s.store.DB.Begin()
	if err != nil {
		s.store.Logf("route changes: could not record line %s: %v", routeShortName, err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM route_changes WHERE detected_at < ?`, cutoff); err != nil {
		s.store.Logf("route changes: could not prune: %v", err)
		return
	}
	// Upstream data that flaps back and forth would otherwise repeat the same news.
	var exists int
	err = tx.QueryRow(
		`SELECT 1 FROM route_changes WHERE route_short_name = ? AND signature = ? LIMIT 1`,
		routeShortName, signature).Scan(&exists)
	if err == nil {
		return
	}
	if err != sql.ErrNoRows {
		s.store.Logf("route changes: could not check line %s: %v", routeShortName, err)
		return
	}
	if _, err := tx.Exec(
		`INSERT INTO route_changes (route_short_name, source, changes, signature, detected_at) VALUES (?, ?, ?, ?, ?)`,
		routeShortName, source, string(payload), signature, now.UnixMilli()); err != nil {
		s.store.Logf("route changes: could not record line %s: %v", routeShortName, err)
		return
	}
	if err := tx.Commit(); err != nil {
		s.store.Logf("route changes: could not record line %s: %v", routeShortName, err)
		return
	}

	kinds := make([]string, len(items))
	for i, item := range items {
		kinds[i] = item.Kind + "/" + item.Direction
	}
	s.store.Logf("route changes: line %s %s changed (%s)", routeShortName, source, strings.Join(kinds, ", "))
}

func (s *Service) routeChangesHandler(c fiber.Ctx) error {
	var changes []RouteChange
	var err error
	if raw := c.Query("id"); raw != "" {
		id, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
		}
		changes, err = s.recentRouteChanges("c.id = ?", id)
	} else {
		changes, err = s.routeChangesOf(routeNamesQuery(c))
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(changes)
}

func routeNamesQuery(c fiber.Ctx) []string {
	var names []string
	for _, name := range strings.Split(c.Query("routes"), ",") {
		if name = strings.TrimSpace(name); name != "" && len(names) < MaxFilterNames {
			names = append(names, name)
		}
	}
	return names
}

func (s *Service) routeChangesOf(names []string) ([]RouteChange, error) {
	if len(names) == 0 {
		return []RouteChange{}, nil
	}
	args := make([]any, len(names))
	for i, name := range names {
		args[i] = name
	}
	return s.recentRouteChanges("c.route_short_name IN ("+strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")+")", args...)
}

func (s *Service) recentRouteChanges(condition string, args ...any) ([]RouteChange, error) {
	args = append([]any{time.Now().Add(-updateRetention).UnixMilli()}, args...)
	args = append(args, maxRouteChangesPerRequest)
	return database.QueryRows(s.store.DB, `
		SELECT c.id, c.route_short_name, COALESCE(r.route_id, 0), COALESCE(r.route_color, ''),
		       c.source, c.detected_at, c.changes
		FROM route_changes c
		LEFT JOIN (SELECT route_short_name, MAX(route_id) AS route_id, route_color FROM routes GROUP BY route_short_name) r
		  ON r.route_short_name = c.route_short_name
		WHERE c.detected_at >= ? AND `+condition+`
		ORDER BY c.detected_at DESC, c.id DESC
		LIMIT ?`,
		args,
		func(rows *sql.Rows) (RouteChange, error) {
			var rc RouteChange
			var payload string
			if err := rows.Scan(&rc.ID, &rc.RouteShortName, &rc.RouteID, &rc.RouteColor, &rc.Source, &rc.DetectedAt, &payload); err != nil {
				return rc, err
			}
			rc.RouteColorful = models.ColorfulRouteColor(rc.RouteShortName)
			rc.Changes = []RouteChangeItem{}
			return rc, json.Unmarshal([]byte(payload), &rc.Changes)
		})
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func reverseSlice[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
