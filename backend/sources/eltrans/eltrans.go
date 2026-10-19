// Package eltrans reads the headways in Eltrans Botoșani's timetable PDF.
package eltrans

import (
	"bytes"
	"conexiuni-cluj/models"
	"conexiuni-cluj/sources"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

type Source struct {
	url     string
	cadence time.Duration
}

func New(url string, cadence time.Duration) *Source {
	return &Source{url: url, cadence: cadence}
}

func (s *Source) Cadence() time.Duration { return s.cadence }

type window struct {
	line       string
	day        string
	startMin   int
	endMin     int
	headwayMin int
	// The text after the headway, naming the variant that runs.
	note string
}

var (
	lineRow   = regexp.MustCompile(`Traseu\s+(\d+)`)
	windowRow = regexp.MustCompile(`(\d{1,2})[,.](\d{2})\s*[–-]\s*(\d{1,2})[,.](\d{2})\s+[Dd]in\s+(\d+)\s+[iî]n\s+\d+\s+min\w*\s*(.*)$`)
)

// A block names its day once, on any of its rows.
func dayOf(block []string) string {
	for _, row := range block {
		switch text := strings.ToLower(strings.TrimSpace(row)); {
		case strings.Contains(text, "luni"):
			return "weekdays"
		case strings.HasPrefix(text, "sambata") || strings.HasPrefix(text, "sâmbăta"):
			return "saturday"
		case strings.HasPrefix(text, "duminica"):
			return "sunday"
		}
	}
	return ""
}

// Blank rows separate the blocks.
func parseWindows(rows []string) []window {
	var out []window
	var block []string
	flush := func() {
		day := dayOf(block)
		line := ""
		for _, row := range block {
			if m := lineRow.FindStringSubmatch(row); m != nil {
				line = m[1]
			}
			m := windowRow.FindStringSubmatch(row)
			if m == nil || line == "" || day == "" {
				continue
			}
			startH, _ := strconv.Atoi(m[1])
			startM, _ := strconv.Atoi(m[2])
			endH, _ := strconv.Atoi(m[3])
			endM, _ := strconv.Atoi(m[4])
			headway, _ := strconv.Atoi(m[5])
			if headway <= 0 {
				continue
			}
			out = append(out, window{line: line, day: day, startMin: startH*60 + startM, endMin: endH*60 + endM, headwayMin: headway, note: strings.TrimSpace(m[6])})
		}
		block = block[:0]
	}
	for _, row := range rows {
		if strings.TrimSpace(row) == "" {
			flush()
			continue
		}
		block = append(block, row)
	}
	flush()
	return out
}

// Windows marked "Util" run line 102's short variant, Tranzy's 102S.
func routeOf(w window) string {
	if w.line == "102" && strings.Contains(strings.ToLower(w.note), "util") {
		return "102S"
	}
	return w.line
}

// The last window's end is the last bus.
func departures(windows []window) []string {
	sort.Slice(windows, func(i, j int) bool { return windows[i].startMin < windows[j].startMin })
	seen := map[int]bool{}
	var minutes []int
	for i, w := range windows {
		last := i == len(windows)-1
		for m := w.startMin; m < w.endMin || (last && m == w.endMin); m += w.headwayMin {
			if !seen[m] {
				seen[m] = true
				minutes = append(minutes, m)
			}
		}
	}
	sort.Ints(minutes)
	out := make([]string, len(minutes))
	for i, m := range minutes {
		out[i] = fmt.Sprintf("%02d:%02d", m/60, m%60)
	}
	return out
}

func frequency(windows []window) *models.Frequency {
	if len(windows) == 0 {
		return nil
	}
	f := &models.Frequency{MinMinutes: windows[0].headwayMin, MaxMinutes: windows[0].headwayMin}
	first, last := windows[0], windows[0]
	for _, w := range windows {
		f.MinMinutes = min(f.MinMinutes, w.headwayMin)
		f.MaxMinutes = max(f.MaxMinutes, w.headwayMin)
		if w.startMin < first.startMin {
			first = w
		}
		if w.endMin > last.endMin {
			last = w
		}
	}
	f.Start = fmt.Sprintf("%02d:%02d", first.startMin/60, first.startMin%60)
	f.End = fmt.Sprintf("%02d:%02d", last.endMin/60, last.endMin%60)
	return f
}

// The PDF gives one headway per line, so both directions share it.
func daySchedule(windows []window) models.DaySchedule {
	times := departures(windows)
	entries := make([]models.TimetableEntry, len(times))
	for i, t := range times {
		entries[i] = models.TimetableEntry{DepartureIn: t, DepartureOut: t}
	}
	f := frequency(windows)
	return models.DaySchedule{Entries: entries, InFrequency: f, OutFrequency: f}
}

func (s *Source) Timetables(snap *sources.Snapshot) (map[string]*models.Timetable, error) {
	rows, err := s.rows()
	if err != nil {
		return nil, err
	}
	byRoute := map[string]map[string][]window{}
	for _, w := range parseWindows(rows) {
		route := routeOf(w)
		if byRoute[route] == nil {
			byRoute[route] = map[string][]window{}
		}
		byRoute[route][w.day] = append(byRoute[route][w.day], w)
	}
	out := map[string]*models.Timetable{}
	for _, route := range snap.Routes {
		days, ok := byRoute[route.RouteShortName]
		if !ok {
			continue
		}
		first := sources.FirstStopNames(snap, route.RouteID)
		out[route.RouteShortName] = &models.Timetable{
			RouteShortName: route.RouteShortName,
			RouteLongName:  route.RouteLongName,
			InStopName:     first[0],
			OutStopName:    first[1],
			Weekdays:       daySchedule(days["weekdays"]),
			Saturday:       daySchedule(days["saturday"]),
			Sunday:         daySchedule(days["sunday"]),
		}
	}
	return out, nil
}

func (s *Source) rows() ([]string, error) {
	data, err := sources.Download(s.url)
	if err != nil {
		return nil, fmt.Errorf("eltrans: %w", err)
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("eltrans: %s: %w", s.url, err)
	}
	var out []string
	for i := 1; i <= reader.NumPage(); i++ {
		rows, err := reader.Page(i).GetTextByRow()
		if err != nil {
			return nil, fmt.Errorf("eltrans: page %d: %w", i, err)
		}
		for _, row := range rows {
			out = append(out, rowText(row.Content))
		}
	}
	return out, nil
}

// Joins a row's glyph runs left to right, with a space only across a real gap.
func rowText(words []pdf.Text) string {
	words = append([]pdf.Text(nil), words...)
	sort.Slice(words, func(a, b int) bool { return words[a].X < words[b].X })
	var sb strings.Builder
	endX := -1.0
	for _, w := range words {
		if endX >= 0 && w.X-endX > w.FontSize/4 {
			sb.WriteByte(' ')
		}
		sb.WriteString(w.S)
		endX = w.X + w.W
	}
	return sb.String()
}
