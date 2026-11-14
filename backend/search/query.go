package search

import (
	"regexp"
	"strings"

	"conexiuni-cluj/router"
)

const (
	showTimetable = "timetable"
	showMap       = "map"
)

type only uint8

const (
	anyKind only = iota
	linesOnly
	stopsOnly
)

// Words that say what to show, not what to find.
var (
	timetableWords = wordSet("orar", "orarul", "orare", "program", "programul", "mersul", "pdf", "plecari", "timetable", "schedule")
	mapWords       = wordSet("live", "harta", "map")
	lineWords      = wordSet("linia", "linie", "liniei", "liniile", "autobuz", "autobuzul", "autobuze", "autobuzelor", "tramvai", "tramvaiul",
		"troleibuz", "troleibuzul", "traseu", "traseul", "trasee", "ruta", "rute", "bus", "line", "tram")
	stopWords   = wordSet("statia", "statie", "statii", "statiei", "statiile", "stop", "station")
	ticketWords = wordSet("bilet", "bilete", "biletul", "biletului", "abonament", "abonamente", "abonamentul", "pret", "pretul", "preturi",
		"tarif", "tarife", "ticket", "tickets", "fare", "price")
)

func wordSet(words ...string) map[string]bool {
	out := make(map[string]bool, len(words))
	for _, w := range words {
		out[w] = true
	}
	return out
}

type reading struct {
	// Folded, with the hint words taken out.
	text    string
	words   []string
	show    string
	only    only
	tickets bool
}

func read(query string) reading {
	r := reading{}
	all := router.Words(query)
	var kept []string
	for _, w := range all {
		switch {
		case timetableWords[w]:
			r.show = showTimetable
		case mapWords[w]:
			r.show = showMap
		case lineWords[w]:
			r.only = linesOnly
		case stopWords[w]:
			r.only = stopsOnly
		case ticketWords[w]:
			r.tickets = true
		default:
			kept = append(kept, w)
		}
	}
	if len(kept) == 0 {
		r.show, r.only = "", anyKind
		kept = all
	}
	r.words = kept
	r.text = strings.Join(kept, " ")
	return r
}

var (
	tripStart = regexp.MustCompile(`^(?:de la|din)\s+`)
	// "in" only after "din" or "de la": names hold it often.
	tripLink   = regexp.MustCompile(`\s+(?:pana la|spre|catre|la)\s+|\s*(?:->|→|>|\x{2013}|\x{2014})\s*|\s+-\s+`)
	tripLinkIn = regexp.MustCompile(`\s+in\s+`)
)

// from is empty for "spre vivo".
func tripEnds(query string) (from, to string, ok bool) {
	q := strings.TrimSpace(router.Fold(query))
	started := tripStart.MatchString(q)
	q = tripStart.ReplaceAllString(q, "")
	if rest, found := strings.CutPrefix(q, "spre "); found {
		return "", rest, true
	}
	if rest, found := strings.CutPrefix(q, "la "); found && !started {
		return "", rest, true
	}
	link := tripLink.FindStringIndex(q)
	if link == nil && started {
		link = tripLinkIn.FindStringIndex(q)
	}
	if link == nil {
		return "", "", false
	}
	from, to = strings.TrimSpace(q[:link[0]]), strings.TrimSpace(q[link[1]:])
	return from, to, from != "" && to != ""
}
