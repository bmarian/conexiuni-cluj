package handlers

import (
	"strings"
	"sync"
	"time"
)

// ProblemLog sits next to the app log and keeps the latest failures for the dashboard.
var ProblemLog = &problemLog{}

const problemLogSize = 40

var problemWords = []string{"failed:", "failed ", "error:", "err=", "panic", "refused", "exceeded", "could not", "warning:"}

type problemLine struct {
	First time.Time
	Last  time.Time
	Count int
	Text  string
	shape string
}

type problemLog struct {
	mu    sync.Mutex
	lines []problemLine
}

func (p *problemLog) Write(b []byte) (int, error) {
	text := strings.TrimSpace(string(b))
	// Drop the "2006/01/02 15:04:05 " the log package puts in front.
	if len(text) > 20 && text[4] == '/' && text[13] == ':' {
		text = text[20:]
	}
	lower := strings.ToLower(text)
	for _, w := range problemWords {
		if strings.Contains(lower, w) {
			p.add(text, time.Now())
			break
		}
	}
	return len(b), nil
}

// Repeats differ only in their numbers; they share one line with a count.
func problemShape(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return '#'
		}
		return r
	}, text)
}

func (p *problemLog) add(text string, now time.Time) {
	shape := problemShape(text)
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, l := range p.lines {
		if l.shape == shape {
			l.Last, l.Text = now, text
			l.Count++
			p.lines = append(append(p.lines[:i:i], p.lines[i+1:]...), l)
			return
		}
	}
	p.lines = append(p.lines, problemLine{First: now, Last: now, Count: 1, Text: text, shape: shape})
	if len(p.lines) > problemLogSize {
		p.lines = p.lines[len(p.lines)-problemLogSize:]
	}
}

// Recent returns the problems seen since `since`, latest first.
func (p *problemLog) Recent(since time.Time) []problemLine {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]problemLine, 0, len(p.lines))
	for i := len(p.lines) - 1; i >= 0; i-- {
		if p.lines[i].Last.After(since) {
			out = append(out, p.lines[i])
		}
	}
	return out
}
