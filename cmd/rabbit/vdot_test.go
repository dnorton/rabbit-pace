package main

import (
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

// Reference times are from the VDOT tables in Daniels' Running Formula.
func TestVDOTMatchesDanielsTables(t *testing.T) {
	tests := []struct {
		dist string
		time string
		vdot float64
	}{
		{"5k", "30:40", 30},
		{"5k", "19:57", 50},
		{"10k", "41:21", 50},
		{"half", "1:31:35", 50},
		{"marathon", "3:10:49", 50},
	}
	for _, tt := range tests {
		m, _ := ParseDistance(tt.dist, Kilometer)
		d, _ := ParseClock(tt.time)
		if got := VDOT(m, d); math.Abs(got-tt.vdot) > 0.2 {
			t.Errorf("VDOT(%s in %s) = %.2f, want %v", tt.dist, tt.time, got, tt.vdot)
		}
	}
}

func TestRaceTimeInvertsVDOT(t *testing.T) {
	for _, m := range []float64{1500, 5000, 21097.5, 42195} {
		for _, v := range []float64{30, 45, 60, 80} {
			got := VDOT(m, RaceTime(v, m))
			if math.Abs(got-v) > 1e-6 {
				t.Errorf("VDOT(%v m, RaceTime(%v)) = %v", m, v, got)
			}
		}
	}
}

func TestEquivTable(t *testing.T) {
	m, total := 8000.0, 33*time.Minute
	tbl := equivTable(VDOT(m, total), m, total, Kilometer, false, false)
	if len(tbl.Rows) != len(equivRaces)+1 {
		t.Fatalf("got %d rows, want %d (standard races plus the 8 km run)", len(tbl.Rows), len(equivRaces)+1)
	}
	var highlighted []string
	for _, r := range tbl.Rows {
		if r.Highlight {
			highlighted = append(highlighted, r.Cells[0]+" "+r.Cells[2])
		}
	}
	if len(highlighted) != 1 || highlighted[0] != "8 km 33:00" {
		t.Errorf("highlighted rows = %q, want just the 8 km run in 33:00", highlighted)
	}

	// A standard distance isn't duplicated, and keeps the actual time.
	tbl = equivTable(VDOT(10000, 45*time.Minute), 10000, 45*time.Minute, Kilometer, false, false)
	if len(tbl.Rows) != len(equivRaces) {
		t.Errorf("10K: got %d rows, want %d", len(tbl.Rows), len(equivRaces))
	}
}

func TestRunVDOTFlags(t *testing.T) {
	var buf strings.Builder
	if err := run([]string{"-time", "45:30", "-dist", "10k", "-vdot", "-only", "-md"}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "- **VDOT:** 44.7") {
		t.Errorf("output missing VDOT line:\n%s", buf.String())
	}

	buf.Reset()
	if err := run([]string{"-time", "45:30", "-dist", "10k", "-equiv", "-md"}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "| Marathon | 42.2 | 3:29:30 |") {
		t.Errorf("-equiv output missing marathon prediction:\n%s", buf.String())
	}

	bad := [][]string{
		{"-time", "2:00", "-dist", "800m", "-vdot"},
		{"-time", "5:00:00", "-dist", "50k", "-equiv"},
		{"-time", "45:30", "-dist", "10k", "-equiv", "-only"},
		{"-time", "45:30", "-dist", "10k", "-equiv", "-every", "5k"},
	}
	for _, args := range bad {
		if err := run(args, io.Discard); err == nil {
			t.Errorf("run(%q) succeeded, want error", args)
		}
	}
}
