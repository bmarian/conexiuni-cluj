package database

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLineTracesKeepOnlyTracedLines(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, filepath.Join(dir, "a.db"), filepath.Join(dir, "b.db"))
	t0 := time.Unix(1_800_000_000, 0)
	if err := s.SetTracedLines([]string{"M6", "M104"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLineFixes([]LineFix{
		{Line: "M6", Vehicle: "b", Lat: 1, First: t0, Last: t0},
		{Line: "M6", Vehicle: "a", Lat: 2, First: t0.Add(time.Hour), Last: t0.Add(2 * time.Hour)},
		{Line: "M6", Vehicle: "a", Lat: 3, First: t0, Last: t0},
		{Line: "M104", Vehicle: "c", First: t0, Last: t0},
	}); err != nil {
		t.Fatal(err)
	}
	fixes, err := s.LineFixes("M6", t0)
	if err != nil {
		t.Fatal(err)
	}
	// By bus, then time.
	if len(fixes) != 3 || fixes[0].Lat != 3 || fixes[1].Lat != 2 || fixes[2].Vehicle != "b" || fixes[1].Last.Sub(fixes[1].First) != time.Hour {
		t.Fatalf("fixes %+v", fixes)
	}
	if err := s.PruneLineFixes("M6", t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if fixes, _ := s.LineFixes("M6", time.Time{}); len(fixes) != 1 {
		t.Fatalf("after prune %+v", fixes)
	}
	if err := s.SetTracedLines([]string{"M6"}); err != nil {
		t.Fatal(err)
	}
	lines, _ := s.TracedLines()
	if fixes, _ := s.LineFixes("M104", time.Time{}); len(fixes) != 0 || len(lines) != 1 {
		t.Fatalf("an untraced line kept %+v, lines %v", fixes, lines)
	}
}
