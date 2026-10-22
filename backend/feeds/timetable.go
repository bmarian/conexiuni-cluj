package feeds

import (
	"conexiuni-cluj/models"
	"fmt"
	"strconv"
	"strings"
)

// Columns lays two directions' departures side by side, row by row.
func Columns(in, out []string) models.DaySchedule {
	entries := make([]models.TimetableEntry, max(len(in), len(out)))
	for i := range entries {
		if i < len(in) {
			entries[i].DepartureIn = in[i]
		}
		if i < len(out) {
			entries[i].DepartureOut = out[i]
		}
	}
	return models.DaySchedule{Entries: entries}
}

// CanonicalTime moves a leading annotation to the end, so "*07:25" becomes "07:25*".
func CanonicalTime(s string) string {
	s = strings.TrimSpace(s)
	bare := strings.TrimLeft(s, "*")
	return bare + s[:len(s)-len(bare)]
}

// NormalizeEntries writes times past midnight as 24+ ("00:30" after "23:30" is "24:30"), each column on its own.
func NormalizeEntries(entries []models.TimetableEntry) {
	prevIn, prevOut := -1, -1
	offIn, offOut := 0, 0
	for i := range entries {
		entries[i].DepartureIn = normalizeTime(entries[i].DepartureIn, &prevIn, &offIn)
		entries[i].DepartureOut = normalizeTime(entries[i].DepartureOut, &prevOut, &offOut)
	}
}

func normalizeTime(s string, prev *int, offset *int) string {
	s = CanonicalTime(s)
	if s == "" {
		return s
	}
	// Split "HH:MM" from any trailing annotation (e.g. "*").
	colon := strings.Index(s, ":")
	if colon < 0 {
		return s
	}
	end := colon + 1
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	timeStr, suffix := s[:end], s[end:]

	parts := strings.SplitN(timeStr, ":", 2)
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return s
	}

	cur := h*60 + m
	// Only a drop of over 12h is midnight; CTP typos like 14:32 among 11:3x are not.
	if *prev >= 0 && *prev-cur > 12*60 {
		*offset += 1440
	}
	*prev = cur

	if *offset == 0 {
		return s
	}
	total := cur + *offset
	return fmt.Sprintf("%02d:%02d%s", total/60, total%60, suffix)
}
