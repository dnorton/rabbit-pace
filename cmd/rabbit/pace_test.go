package main

import (
	"math"
	"testing"
	"time"
)

func TestParseDistance(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"10k", 10000},
		{"5km", 5000},
		{"800m", 800},
		{"1mi", 1609.344},
		{"13.1 miles", 13.1 * 1609.344},
		{"marathon", 42195},
		{"Half", 21097.5},
		{"10", 10000}, // bare number uses the default unit
	}
	for _, tt := range tests {
		got, err := ParseDistance(tt.in, Kilometer)
		if err != nil {
			t.Errorf("ParseDistance(%q) error: %v", tt.in, err)
			continue
		}
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("ParseDistance(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	for _, bad := range []string{"", "abc", "10furlongs", "-5k", "0k"} {
		if _, err := ParseDistance(bad, Kilometer); err == nil {
			t.Errorf("ParseDistance(%q) expected error", bad)
		}
	}
}

func TestParseClock(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"4:30", 4*time.Minute + 30*time.Second},
		{"1:45:00", time.Hour + 45*time.Minute},
		{"90", 90 * time.Second},
		{"45m30s", 45*time.Minute + 30*time.Second},
	}
	for _, tt := range tests {
		got, err := ParseClock(tt.in)
		if err != nil {
			t.Errorf("ParseClock(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseClock(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	for _, bad := range []string{"", "abc", "1:2:3:4", "0:00", "-4:30"} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("ParseClock(%q) expected error", bad)
		}
	}
}

func TestFormatClock(t *testing.T) {
	tests := map[time.Duration]string{
		4*time.Minute + 30*time.Second:            "4:30",
		time.Hour + 2*time.Minute + 3*time.Second: "1:02:03",
		59*time.Second + 600*time.Millisecond:     "1:00",
	}
	for in, want := range tests {
		if got := FormatClock(in); got != want {
			t.Errorf("FormatClock(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestPaceFor(t *testing.T) {
	total := 50 * time.Minute
	if got := FormatClock(PaceFor(total, 10000, Kilometer)); got != "5:00" {
		t.Errorf("pace /km = %s, want 5:00", got)
	}
	if got := FormatClock(PaceFor(total, 10000, Mile)); got != "8:03" {
		t.Errorf("pace /mi = %s, want 8:03", got)
	}
}

func TestSplits(t *testing.T) {
	pace := 5 * time.Minute
	splits := Splits(pace, Kilometer, 1000, 2500, 0)
	if len(splits) != 3 {
		t.Fatalf("got %d splits, want 3", len(splits))
	}
	last := splits[2]
	if last.Meters != 2500 {
		t.Errorf("last split distance = %v, want 2500", last.Meters)
	}
	if last.Time != 150*time.Second {
		t.Errorf("last split time = %v, want 2m30s", last.Time)
	}
	if last.Elapsed != 12*time.Minute+30*time.Second {
		t.Errorf("total = %v, want 12m30s", last.Elapsed)
	}

	// An exact multiple shouldn't produce a zero-length trailing split.
	if n := len(Splits(pace, Kilometer, 1000, 5000, 0)); n != 5 {
		t.Errorf("5k splits = %d, want 5", n)
	}
}

func TestSplitsEvery(t *testing.T) {
	// 5 min/km over a marathon in 5k splits: 8 full splits plus 2.195km.
	splits := Splits(5*time.Minute, Kilometer, 5000, 42195, 0)
	if len(splits) != 9 {
		t.Fatalf("got %d splits, want 9", len(splits))
	}
	if splits[0].Time != 25*time.Minute {
		t.Errorf("first split time = %v, want 25m", splits[0].Time)
	}
	last := splits[8]
	if last.Meters != 42195 {
		t.Errorf("last split distance = %v, want 42195", last.Meters)
	}
	if got := FormatClock(last.Elapsed); got != "3:30:59" {
		t.Errorf("total = %s, want 3:30:59", got)
	}

	// Mile splits with a per-km pace.
	splits = Splits(5*time.Minute, Kilometer, Mile.Meters, 5000, 0)
	if len(splits) != 4 {
		t.Fatalf("got %d mile splits, want 4", len(splits))
	}
	if got := FormatClock(splits[0].Time); got != "8:03" {
		t.Errorf("mile split time = %s, want 8:03", got)
	}
}

func TestSplitsSkew(t *testing.T) {
	tests := []struct {
		name        string
		skew        float64
		first, last string
	}{
		{"negative", 0.05, "5:15", "4:45"},
		{"positive", -0.05, "4:45", "5:15"},
		{"even", 0, "5:00", "5:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splits := Splits(5*time.Minute, Kilometer, 1000, 10000, tt.skew)
			if got := FormatClock(splits[0].Pace); got != tt.first {
				t.Errorf("first split pace = %s, want %s", got, tt.first)
			}
			if got := FormatClock(splits[9].Pace); got != tt.last {
				t.Errorf("last split pace = %s, want %s", got, tt.last)
			}
			if got := FormatClock(splits[9].Elapsed); got != "50:00" {
				t.Errorf("finish = %s, want 50:00", got)
			}
		})
	}

	// A short final split still finishes on the target time.
	splits := Splits(5*time.Minute, Kilometer, 1000, 21097.5, 0.05)
	if got := FormatClock(splits[len(splits)-1].Elapsed); got != "1:45:29" {
		t.Errorf("half finish = %s, want 1:45:29", got)
	}
}

func TestSplitAt(t *testing.T) {
	// Even 5:00/km marathon in 5k splits: halfway falls inside the 25k split.
	splits := Splits(5*time.Minute, Kilometer, 5000, 42195, 0)
	half, i := SplitAt(splits, 42195/2.0)
	if i != 4 {
		t.Errorf("halfway split index = %d, want 4", i)
	}
	if got := FormatClock(half.Elapsed); got != "1:45:29" {
		t.Errorf("halfway time = %s, want 1:45:29", got)
	}

	// A 10k in 5k splits: halfway lands exactly on the first split.
	splits = Splits(5*time.Minute, Kilometer, 5000, 10000, 0.05)
	half, i = SplitAt(splits, 5000)
	if i != 0 || half.Elapsed != splits[0].Elapsed {
		t.Errorf("SplitAt(5000) = %v at %d, want first split %v", half.Elapsed, i, splits[0].Elapsed)
	}
}

func TestMPH(t *testing.T) {
	tests := []struct {
		pace time.Duration
		u    Unit
		want float64
	}{
		{7 * time.Minute, Mile, 60.0 / 7},
		{6 * time.Minute, Mile, 10},
		{5 * time.Minute, Kilometer, 12 / 1.609344},
	}
	for _, tt := range tests {
		if got := MPH(tt.pace, tt.u); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("MPH(%v /%s) = %v, want %v", tt.pace, tt.u.Name, got, tt.want)
		}
	}
}

func TestTimeForAndDistanceFor(t *testing.T) {
	// 7:00/mi over a half marathon.
	if got := FormatClock(TimeFor(7*time.Minute, Mile, 21097.5)); got != "1:31:46" {
		t.Errorf("TimeFor = %s, want 1:31:46", got)
	}
	// 30 minutes at 5:00/km is 6 km.
	if got := DistanceFor(30*time.Minute, 5*time.Minute, Kilometer); math.Abs(got-6000) > 1e-9 {
		t.Errorf("DistanceFor = %v, want 6000", got)
	}
	// Round trip: pace from the time and distance we just computed. Float
	// rounding can leave it a nanosecond out, so allow a little slack.
	meters := DistanceFor(45*time.Minute, 7*time.Minute, Mile)
	if got := PaceFor(45*time.Minute, meters, Mile); (got - 7*time.Minute).Abs() > time.Microsecond {
		t.Errorf("round trip pace = %v, want 7m", got)
	}
}
