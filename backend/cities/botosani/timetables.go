package botosani

import (
	"bytes"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

type lineProgram struct {
	// By end departed from, kind of day, bus.
	from    map[string]map[string][][]departure
	variant string
}

type departure struct {
	minute  int
	variant bool
}

type cell struct {
	x    float64
	text string
}

var (
	programTitle  = regexp.MustCompile(`pe linia\s+(\S+)\s+cu plecare\s+(?:de la|din)\s+(.+?)\s*\(([^)]*)\)`)
	variantLegend = regexp.MustCompile(`^\*\s*Ruta\s*\(\s*(\w+)\s*\)`)
	departureCell = regexp.MustCompile(`^(\*?)(\d{1,2})[:.](\d{2})$`)
	rowNumber     = regexp.MustCompile(`^\d+$`)
)

func parseProgram(rows [][]cell) map[string]*lineProgram {
	out := map[string]*lineProgram{}
	var line *lineProgram
	var days []string
	var from string
	var columns []float64
	var buses [][]departure
	flush := func() {
		for _, day := range days {
			if line.from[from] == nil {
				line.from[from] = map[string][][]departure{}
			}
			line.from[from][day] = buses
		}
		days, columns, buses = nil, nil, nil
	}
	for _, row := range rows {
		text := joinCells(row)
		if m := programTitle.FindStringSubmatch(text); m != nil {
			if line != nil {
				flush()
			}
			if out[m[1]] == nil {
				out[m[1]] = &lineProgram{from: map[string]map[string][][]departure{}}
			}
			line, from, days = out[m[1]], m[2], kindsOfDay(m[3])
			continue
		}
		if line == nil || len(row) == 0 {
			continue
		}
		if m := variantLegend.FindStringSubmatch(text); m != nil {
			line.variant = m[1]
			continue
		}
		if strings.HasPrefix(text, "Nr") {
			columns = columns[:0]
			for _, c := range row {
				if strings.HasPrefix(c.text, "Nr") || strings.EqualFold(c.text, "BUS") || strings.HasPrefix(strings.ToUpper(c.text), "BUS ") {
					columns = append(columns, c.x)
				}
			}
			buses = make([][]departure, max(len(columns)-1, 0))
			continue
		}
		if len(columns) < 2 || !rowNumber.MatchString(row[0].text) {
			continue
		}
		for _, c := range row[1:] {
			m := departureCell.FindStringSubmatch(c.text)
			bus := nearest(columns, c.x) - 1
			if m == nil || bus < 0 {
				continue
			}
			h, _ := strconv.Atoi(m[2])
			minute, _ := strconv.Atoi(m[3])
			buses[bus] = append(buses[bus], departure{minute: h*60 + minute, variant: m[1] != ""})
		}
	}
	if line != nil {
		flush()
	}
	return out
}

func kindsOfDay(label string) []string {
	label = feeds.NormalizeStopLabel(label)
	var out []string
	for word, day := range map[string]string{"luni": calendar.Weekday, "sambata": calendar.Saturday, "duminica": calendar.Sunday} {
		if strings.Contains(label, word) {
			out = append(out, day)
		}
	}
	return out
}

func nearest(columns []float64, x float64) int {
	best := 0
	for i, c := range columns {
		if math.Abs(c-x) < math.Abs(columns[best]-x) {
			best = i
		}
	}
	return best
}

func readProgram(data []byte) ([][]cell, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out [][]cell
	for i := 1; i <= reader.NumPage(); i++ {
		rows, err := reader.Page(i).GetTextByRow()
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i, err)
		}
		for _, row := range rows {
			out = append(out, cells(row.Content))
		}
	}
	return out, nil
}

func cells(words []pdf.Text) []cell {
	words = append([]pdf.Text(nil), words...)
	sort.SliceStable(words, func(a, b int) bool { return words[a].X < words[b].X })
	var out []cell
	var sb strings.Builder
	start, endX := 0.0, -1.0
	flush := func() {
		if text := strings.Join(strings.Fields(sb.String()), " "); text != "" {
			out = append(out, cell{x: start, text: text})
		}
		sb.Reset()
	}
	for _, w := range words {
		if endX >= 0 && w.X-endX > w.FontSize/4 {
			flush()
		}
		if sb.Len() == 0 {
			start = w.X
		}
		sb.WriteString(w.S)
		endX = w.X + w.W
	}
	flush()
	return out
}

func joinCells(row []cell) string {
	texts := make([]string, len(row))
	for i, c := range row {
		texts[i] = c.text
	}
	return strings.Join(texts, " ")
}
