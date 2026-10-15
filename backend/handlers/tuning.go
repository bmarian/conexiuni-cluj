package handlers

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"
)

var vacationWindowCandidates = []int{3, 5, 7, 10}

const (
	vacationWindowTuning = "weekday_vacation_window"
	tuningEvalDays       = 10
	tuningMinDays        = 6
	tuningMinGain        = 0.02
)

type windowTuning struct {
	Window   int                `json:"window"`
	Scores   map[string]float64 `json:"scores"`
	Baseline float64            `json:"baseline"`
	Days     int                `json:"days"`
	Samples  int                `json:"samples"`
	Changed  bool               `json:"changed"`
	At       int64              `json:"at"`
}

func LoadWindowTuning() {
	t, ok, err := savedWindowTuning()
	if err != nil {
		log.Printf("tuning: load failed: %v", err)
		return
	}
	if ok && slices.Contains(vacationWindowCandidates, t.Window) {
		setDayWindowSize(calendar.WeekdayVacation, t.Window)
	}
}

func savedWindowTuning() (windowTuning, bool, error) {
	var t windowTuning
	data, ok, err := database.Tuning(vacationWindowTuning)
	if err != nil || !ok {
		return t, false, err
	}
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return t, false, err
	}
	return t, true, nil
}

func evaluateVacationWindows(cal *calendar.Calendar, today time.Time) (windowTuning, error) {
	result := windowTuning{Scores: map[string]float64{}}
	longest := slices.Max(vacationWindowCandidates)
	dates := daysOfType(cal, today, calendar.WeekdayVacation, tuningEvalDays+longest)
	days := newReplayDays()

	fallback := map[replayRoute][]segmentProfileRow{}
	fallbackFor := func(route replayRoute) ([]segmentProfileRow, error) {
		if rows, ok := fallback[route]; ok {
			return rows, nil
		}
		all, err := loadSegmentProfileRows(int(route.route), int(route.direction))
		if err != nil {
			return nil, err
		}
		rows := slices.DeleteFunc(all, func(r segmentProfileRow) bool { return r.dayType != calendar.Weekday })
		fallback[route] = rows
		return rows, nil
	}

	sums := map[int]float64{}
	baseline := 0.0
	for i := 0; i < len(dates) && result.Days < tuningEvalDays; i++ {
		samples, err := days.day(dates[i])
		if err != nil {
			return result, err
		}
		if len(samples) == 0 {
			continue
		}
		term := map[replayRoute][]segmentProfileRow{}
		for _, s := range samples {
			if _, ok := term[s.route]; !ok {
				rows, err := fallbackFor(s.route)
				if err != nil {
					return result, err
				}
				term[s.route] = rows
			}
		}
		termOnly := replayPredictor(term, calendar.WeekdayVacation)
		predictors := map[int]func(replaySample) (float64, bool){}
		for _, n := range vacationWindowCandidates {
			train, err := days.days(dates[i+1 : min(i+1+n, len(dates))])
			if err != nil {
				return result, err
			}
			predictors[n] = replayPredictor(mergeProfileRows(replayProfiles(train, calendar.WeekdayVacation), term), calendar.WeekdayVacation)
		}
		counted := 0
		for _, s := range samples {
			base, ok := termOnly(s)
			if !ok {
				continue
			}
			counted++
			baseline += replayError(s, base)
			for n, predict := range predictors {
				estimate, _ := predict(s)
				sums[n] += replayError(s, estimate)
			}
		}
		if counted > 0 {
			result.Days++
			result.Samples += counted
		}
	}
	if result.Samples == 0 {
		return result, nil
	}
	result.Baseline = baseline / float64(result.Samples)
	for _, n := range vacationWindowCandidates {
		result.Scores[strconv.Itoa(n)] = sums[n] / float64(result.Samples)
	}
	return result, nil
}

func pickVacationWindow(result windowTuning, current int) (int, bool) {
	if result.Days < tuningMinDays {
		return current, false
	}
	score := func(n int) float64 {
		s, ok := result.Scores[strconv.Itoa(n)]
		if !ok {
			return replayErrorCapSec
		}
		return s
	}
	best := current
	for _, n := range vacationWindowCandidates {
		if score(n) < score(best) {
			best = n
		}
	}
	if best != current && score(best) < score(current)*(1-tuningMinGain) {
		return best, true
	}
	return current, false
}

func describeTuning(t windowTuning) string {
	parts := make([]string, 0, len(vacationWindowCandidates))
	for _, n := range vacationWindowCandidates {
		if s, ok := t.Scores[strconv.Itoa(n)]; ok {
			parts = append(parts, fmt.Sprintf("%d days %.0f s", n, s))
		}
	}
	return fmt.Sprintf("%s, term profile alone %.0f s, over %d days and %d stop-to-stop times", strings.Join(parts, ", "), t.Baseline, t.Days, t.Samples)
}

func tuneVacationWindow(now time.Time) {
	if database.Archive == nil {
		return
	}
	cal := currentCalendar()
	today := localMidnight(now)
	if cal.ProfileDay(today.AddDate(0, 0, -1)) != calendar.WeekdayVacation {
		return
	}
	started := time.Now()
	result, err := evaluateVacationWindows(cal, today)
	if err != nil {
		log.Printf("tuning: school-holiday window check failed: %v", err)
		return
	}
	current, _ := dayWindowSize(calendar.WeekdayVacation)
	result.Window, result.Changed = pickVacationWindow(result, current)
	result.At = now.Unix()
	data, err := json.Marshal(result)
	if err == nil {
		err = database.SaveTuning(vacationWindowTuning, string(data), now)
	}
	if err != nil {
		log.Printf("tuning: saving the school-holiday window failed: %v", err)
		return
	}
	log.Printf("tuning: school-holiday window %d (was %d): %s, in %s", result.Window, current, describeTuning(result), time.Since(started).Round(time.Millisecond))
	if result.Changed {
		setDayWindowSize(calendar.WeekdayVacation, result.Window)
		applyCalendar(now)
	}
}
