package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Unit is a distance unit used for paces and splits.
type Unit struct {
	Name   string  // short label, e.g. "km"
	Meters float64 // length of one unit in meters
}

var (
	Kilometer = Unit{Name: "km", Meters: 1000}
	Mile      = Unit{Name: "mi", Meters: 1609.344}
)

// ParseUnit accepts "km", "k", "kilometers", "mi", "mile", "miles", etc.
func ParseUnit(s string) (Unit, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "km", "k", "kms", "kilometer", "kilometers", "kilometre", "kilometres":
		return Kilometer, nil
	case "mi", "mile", "miles":
		return Mile, nil
	}
	return Unit{}, fmt.Errorf("unknown unit %q (use km or mi)", s)
}

// namedDistances are common race distances, in meters.
var namedDistances = map[string]float64{
	"marathon": 42195,
	"half":     21097.5,
}

// ParseDistance parses a distance like "10k", "5km", "13.1mi", "800m",
// "half" or "marathon" and returns it in meters. A bare number such as
// "10" is interpreted in defaultUnit.
func ParseDistance(s string, defaultUnit Unit) (float64, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if m, ok := namedDistances[s]; ok {
		return m, nil
	}

	// Split into the numeric prefix and the unit suffix.
	i := strings.IndexFunc(s, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	numStr, suffix := s, ""
	if i >= 0 {
		numStr, suffix = s[:i], strings.TrimSpace(s[i:])
	}

	n, err := strconv.ParseFloat(numStr, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid distance %q", s)
	}

	switch suffix {
	case "":
		return n * defaultUnit.Meters, nil
	case "m", "meters", "metres":
		return n, nil
	}
	u, err := ParseUnit(suffix)
	if err != nil {
		return 0, fmt.Errorf("invalid distance %q: %w", s, err)
	}
	return n * u.Meters, nil
}

// ParseClock parses "ss", "mm:ss" or "hh:mm:ss" into a duration. It also
// accepts Go duration syntax such as "45m30s".
func ParseClock(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, ":") {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d, nil
		}
	}

	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("invalid time %q (use hh:mm:ss or mm:ss)", s)
	}
	var total float64
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid time %q (use hh:mm:ss or mm:ss)", s)
		}
		total = total*60 + v
	}
	if total <= 0 {
		return 0, fmt.Errorf("time %q must be greater than zero", s)
	}
	return time.Duration(total * float64(time.Second)), nil
}

// FormatClock formats a duration as "m:ss" or "h:mm:ss", rounded to the
// nearest second.
func FormatClock(d time.Duration) string {
	secs := int(math.Round(d.Seconds()))
	h, m, s := secs/3600, secs/60%60, secs%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// PaceFor returns the time per unit for covering meters in total time.
func PaceFor(total time.Duration, meters float64, u Unit) time.Duration {
	return time.Duration(float64(total) * u.Meters / meters)
}

// TimeFor returns how long meters takes at pace (time per unit u).
func TimeFor(pace time.Duration, u Unit, meters float64) time.Duration {
	return time.Duration(float64(pace) * meters / u.Meters)
}

// DistanceFor returns how many meters are covered in total time at pace
// (time per unit u).
func DistanceFor(total, pace time.Duration, u Unit) float64 {
	return float64(total) / float64(pace) * u.Meters
}

// MPH converts a pace (time per unit u) to a speed in miles per hour.
func MPH(pace time.Duration, u Unit) float64 {
	return u.Meters / Mile.Meters / pace.Hours()
}

// Split is one row of a splits table.
type Split struct {
	Number  int
	Meters  float64       // cumulative distance at the end of this split
	Pace    time.Duration // pace for this split, per the pace unit
	Time    time.Duration // time for this split alone
	Elapsed time.Duration // cumulative time at the end of this split
}

// Splits breaks meters into one split every interval meters, plus a final
// partial split if the distance doesn't divide evenly. pace is the target
// average time per unit u.
//
// skew shapes the pacing: 0 is even pace, 0.05 is a 5% negative split
// (first split 5% slower, last 5% faster, stepping evenly in between) and
// -0.05 is a 5% positive split. The finish time always matches pace.
func Splits(pace time.Duration, u Unit, interval, meters, skew float64) []Split {
	// Work out where each split ends.
	var ends []float64
	for covered := 0.0; covered < meters; {
		next := math.Min(covered+interval, meters)
		// Treat a tiny floating point leftover as the end of the run.
		if meters-next < 0.001 {
			next = meters
		}
		ends = append(ends, next)
		covered = next
	}

	// Give each split a pace multiplier stepping from 1+skew to 1-skew,
	// then scale them so the total time matches the target. The scaling
	// only matters when a short final split unbalances the steps.
	n := len(ends)
	perMeter := float64(pace) / u.Meters
	factors := make([]float64, n)
	var rawTotal float64
	start := 0.0
	for i, end := range ends {
		factors[i] = 1
		if n > 1 {
			factors[i] = 1 + skew*(1-2*float64(i)/float64(n-1))
		}
		rawTotal += (end - start) * factors[i]
		start = end
	}
	scale := meters / rawTotal

	out := make([]Split, n)
	var elapsed time.Duration
	start = 0.0
	for i, end := range ends {
		splitPerMeter := perMeter * factors[i] * scale
		t := time.Duration((end - start) * splitPerMeter)
		elapsed += t
		out[i] = Split{
			Number:  i + 1,
			Meters:  end,
			Pace:    time.Duration(splitPerMeter * u.Meters),
			Time:    t,
			Elapsed: elapsed,
		}
		start = end
	}
	return out
}

// SplitAt returns a row for the point at meters along the run, such as
// halfway, using the pace of the split that contains it. It also returns
// the index of that split. Time is left as zero since the point isn't a
// full split.
func SplitAt(splits []Split, at float64) (Split, int) {
	start, elapsed := 0.0, time.Duration(0)
	for i, s := range splits {
		if at <= s.Meters+0.001 || i == len(splits)-1 {
			frac := (at - start) / (s.Meters - start)
			return Split{
				Meters:  at,
				Pace:    s.Pace,
				Elapsed: elapsed + time.Duration(frac*float64(s.Time)),
			}, i
		}
		start, elapsed = s.Meters, s.Elapsed
	}
	return Split{}, -1
}
