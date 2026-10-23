package planner

import (
	"testing"
	"time"
)

func TestPlansRunOneAtATimePerClient(t *testing.T) {
	release, ok := acquirePlanSlot("a")
	if !ok {
		t.Fatal("first plan refused")
	}
	second := make(chan func())
	go func() {
		r, _ := acquirePlanSlot("a")
		second <- r
	}()
	if cap(planSlots) > 1 {
		r, ok := acquirePlanSlot("b")
		if !ok {
			t.Fatal("another client was held up")
		}
		r()
	}
	select {
	case <-second:
		t.Fatal("a second plan of the same client ran alongside the first")
	case <-time.After(100 * time.Millisecond):
	}
	release()
	select {
	case r := <-second:
		r()
	case <-time.After(time.Second):
		t.Fatal("the client's next plan never started")
	}
	planTurns.Lock()
	left := len(planTurns.byClient)
	planTurns.Unlock()
	if left != 0 {
		t.Fatalf("%d clients still listed after their plans ended", left)
	}
}

func TestLivePlanDay(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Bucharest")
	morning := time.Date(2026, 10, 12, 9, 0, 0, 0, loc)
	evening := time.Date(2026, 10, 12, 21, 0, 0, 0, loc)
	cases := []struct {
		key  string
		now  time.Time
		want bool
	}{
		{"2026-10-12", morning, true},
		{"2026-10-13", morning, false},
		{"2026-10-13", evening, true},
		{"2026-10-11", morning, false},
		{"9999-01-01", morning, false},
	}
	for _, tc := range cases {
		if got := livePlanDay(tc.key, tc.now, 8*time.Hour); got != tc.want {
			t.Errorf("livePlanDay(%s, %s) = %v, want %v", tc.key, tc.now.Format("15:04"), got, tc.want)
		}
	}
}

func TestPlacesWhileTheWalkGraphIsRead(t *testing.T) {
	s := &Service{}
	s.mu.Lock()
	defer s.mu.Unlock()
	read := make(chan struct{})
	go func() {
		s.Places()
		close(read)
	}()
	select {
	case <-read:
	case <-time.After(time.Second):
		t.Fatal("Places waits for the walk graph's lock")
	}
}
