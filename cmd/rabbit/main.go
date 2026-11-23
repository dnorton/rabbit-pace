// Command rabbit is a running pace calculator, named after the pacers
// who set the tempo in a race.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `rabbit - running pace calculator

Give any two of time, pace and distance. rabbit works out the third and
prints a split table.

Usage:
  rabbit [two of: -time <time>  -pace <pace>  -dist <distance>]
         [-unit km|mi] [-every <distance>] [-halfway]
         [-neg <percent> | -pos <percent>] [-mph] [-kph] [-md] [-only]
         [-fuel <time|distance>] [-vdot] [-equiv]
  rabbit version

Flags:
  -time      finish time
  -pace      pace per -unit
  -dist      distance
  -unit      km or mi (default km): the unit of -pace, of a bare -dist or
             -every number, and of the split table
  -every     split length, e.g. 5k or 1mi (default: one -unit)
  -halfway   add a row at the halfway point
  -fuel      add a row for each fuel stop (gel, drink), every so much
             time (30min, 1h, 45:00) or distance (8k, 5mi)
  -neg 5     negative split: start 5% slower, finish 5% faster
  -pos 5     positive split: start 5% faster, finish 5% slower
             (the finish time stays the same either way)
  -mph       add speed in miles per hour, for treadmills
  -kph       add speed in kilometers per hour, for treadmills
  -md        print Markdown, ready to paste into Obsidian or a README
  -only      print just the summary, without the split table
  -vdot      add the VDOT fitness score (Jack Daniels) for a race result
             between 1500 m and the marathon
  -equiv     show predicted times at other race distances, from VDOT,
             instead of the split table

Formats:
  time/pace  hh:mm:ss, mm:ss, or Go durations like 45m30s
  distance   10k, 5km, 13.1mi, 800m, half, marathon, or a bare number in -unit

Examples:
  rabbit -time 45:30 -dist 10k -only
  rabbit -pace 7:00 -unit mi -dist half
  rabbit -pace 5:00 -time 30:00
  rabbit -pace 4:30 -dist marathon -every 5k
  rabbit -time 3:00:00 -dist marathon -every 5k -halfway -neg 2
  rabbit -pace 7:00 -unit mi -dist 10k -md
  rabbit -time 3:30:00 -dist marathon -every 5k -fuel 30min
  rabbit -time 45:30 -dist 10k -equiv
`

func main() {
	// colorprofile drops or downgrades colours to suit the terminal, and
	// strips them entirely when output is piped or NO_COLOR is set.
	out := colorprofile.NewWriter(os.Stdout, os.Environ())
	if err := run(os.Args[1:], out); err != nil {
		fmt.Fprintln(os.Stderr, "rabbit:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return nil
	}
	switch args[0] {
	case "version", "-version", "--version":
		fmt.Fprintln(out, "rabbit", currentVersion())
		return nil
	case "help", "-h", "-help", "--help":
		fmt.Fprint(out, usage)
		return nil
	case "pace", "splits":
		// Removed in v0.4.0; point anyone with old commands at the new form.
		hint := "rabbit " + strings.Join(args[1:], " ")
		if args[0] == "pace" {
			hint += " -only"
		}
		return fmt.Errorf("the %q command was removed in v0.4.0; drop it and use flags directly, e.g. %s", args[0], hint)
	}
	return runCalc(args, out)
}

// runCalc works out whichever of time, pace and distance wasn't given,
// then prints a summary and (unless -only) a split table.
func runCalc(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("rabbit", flag.ContinueOnError)
	timeStr := fs.String("time", "", "finish time, e.g. 45:30 or 3:00:00")
	paceStr := fs.String("pace", "", "pace per -unit, e.g. 4:30")
	distStr := fs.String("dist", "", "distance, e.g. 10k, 13.1mi, marathon")
	unitStr := fs.String("unit", "km", "km or mi: unit of -pace, of bare distances, and of the splits")
	everyStr := fs.String("every", "", "split length, e.g. 5k or 1mi (default: one -unit)")
	halfway := fs.Bool("halfway", false, "add a row at the halfway point")
	neg := fs.Float64("neg", 0, "negative split: start this percent slower, finish this percent faster")
	pos := fs.Float64("pos", 0, "positive split: start this percent faster, finish this percent slower")
	mph := fs.Bool("mph", false, "add speed in miles per hour")
	kph := fs.Bool("kph", false, "add speed in kilometers per hour")
	md := fs.Bool("md", false, "print Markdown (e.g. for Obsidian) instead of styled output")
	only := fs.Bool("only", false, "print just the summary, without the split table")
	fuelStr := fs.String("fuel", "", "add a fuel row every so often, e.g. 30min (time) or 8k (distance)")
	vdot := fs.Bool("vdot", false, "add the VDOT fitness score for this race result")
	equiv := fs.Bool("equiv", false, "show equivalent times at other race distances instead of splits")
	// Report flag errors ourselves, as one line, rather than letting the
	// flag package print them along with the whole usage text.
	fs.SetOutput(io.Discard)
	if err := parseFlags(fs, args); err != nil {
		if errors.Is(err, flag.ErrHelp) { // -h anywhere
			fmt.Fprint(out, usage)
			return nil
		}
		return err
	}

	given := 0
	for _, v := range []string{*timeStr, *paceStr, *distStr} {
		if v != "" {
			given++
		}
	}
	if given != 2 {
		return errors.New("give exactly two of -time, -pace and -dist; rabbit works out the third")
	}
	if *only && (*everyStr != "" || *halfway || *fuelStr != "") {
		return errors.New("-every, -halfway and -fuel change the split table, which -only leaves out")
	}
	if *equiv && (*only || *everyStr != "" || *halfway || *fuelStr != "") {
		return errors.New("-equiv replaces the split table, so it can't be used with -only, -every, -halfway or -fuel")
	}
	if *neg != 0 && *pos != 0 {
		return errors.New("use either -neg or -pos, not both")
	}
	for _, p := range []float64{*neg, *pos} {
		if p < 0 || p > 50 {
			return fmt.Errorf("split percent %g must be between 0 and 50", p)
		}
	}
	skew := (*neg - *pos) / 100

	unit, err := ParseUnit(*unitStr)
	if err != nil {
		return err
	}
	var total, pace time.Duration
	var meters float64
	if *timeStr != "" {
		if total, err = ParseClock(*timeStr); err != nil {
			return err
		}
	}
	if *paceStr != "" {
		if pace, err = ParseClock(*paceStr); err != nil {
			return err
		}
	}
	if *distStr != "" {
		if meters, err = ParseDistance(*distStr, unit); err != nil {
			return err
		}
	}
	var fuel FuelEvery
	if *fuelStr != "" {
		if fuel, err = ParseFuel(*fuelStr, unit); err != nil {
			return err
		}
	}
	interval := unit.Meters
	if *everyStr != "" {
		if interval, err = ParseDistance(*everyStr, unit); err != nil {
			return err
		}
	}

	// Work out whichever of the three wasn't given.
	switch {
	case *paceStr == "":
		pace = PaceFor(total, meters, unit)
	case *timeStr == "":
		total = TimeFor(pace, unit, meters)
	default:
		meters = DistanceFor(total, pace, unit)
	}

	splits := Splits(pace, unit, interval, meters, skew)
	first, last := splits[0], splits[len(splits)-1]

	// paces formats a pace per unit in both km and mi.
	paces := func(p time.Duration) (string, string) {
		return FormatClock(PaceFor(p, unit.Meters, Kilometer)), FormatClock(PaceFor(p, unit.Meters, Mile))
	}
	mphOf := func(p time.Duration) string { return fmt.Sprintf("%.1f", MPH(p, unit)) }
	kphOf := func(p time.Duration) string { return fmt.Sprintf("%.1f", KPH(p, unit)) }
	// trend compares a split's pace to the target, ignoring differences
	// too small to show once rounded to the second.
	trend := func(p time.Duration) int {
		switch d := p - pace; {
		case d < -time.Second/2:
			return -1
		case d > time.Second/2:
			return 1
		}
		return 0
	}

	km, mi := paces(pace)
	r := Report{Summary: []Field{
		{Label: "Distance", Value: bothDist(meters), Calculated: *distStr == ""},
		{Label: "Time", Value: FormatClock(total), Calculated: *timeStr == ""},
		{Label: "Pace", Value: km + " /km · " + mi + " /mi", Calculated: *paceStr == ""},
	}}
	var speeds []string
	if *mph {
		speeds = append(speeds, mphOf(pace)+" mph")
	}
	if *kph {
		speeds = append(speeds, kphOf(pace)+" km/h")
	}
	if len(speeds) > 0 {
		r.Summary = append(r.Summary, Field{Label: "Speed", Value: strings.Join(speeds, " · ")})
	}
	if skew != 0 {
		label, pct := "Negative split", *neg
		if skew < 0 {
			label, pct = "Positive split", *pos
		}
		firstKm, firstMi := paces(first.Pace)
		lastKm, lastMi := paces(last.Pace)
		r.Summary = append(r.Summary, Field{
			Label: fmt.Sprintf("%s %g%%", label, pct),
			Value: firstKm + " → " + lastKm + " /km · " + firstMi + " → " + lastMi + " /mi",
		})
	}
	if *vdot || *equiv {
		if err := checkVDOTRange(meters); err != nil {
			return err
		}
		score := VDOT(meters, total)
		r.Summary = append(r.Summary, Field{Label: "VDOT", Value: fmt.Sprintf("%.1f", score)})
		if *equiv {
			r.Table = equivTable(score, meters, total, unit, *mph, *kph)
			return r.write(out, *md)
		}
	}
	if *only {
		return r.write(out, *md)
	}

	// Marker rows (halfway, fuel stops) sit between the splits. One that
	// falls on a split boundary labels that split instead of adding a row.
	var markers []Marker
	if *halfway {
		markers = append(markers, Marker{Label: "Half", Short: "half", Meters: meters / 2})
	}
	markers = append(markers, fuelMarkers(fuel, splits)...)
	sortMarkers(markers)

	t := &Table{
		Headers:   []string{"Split", "Distance (" + unit.Name + ")", "Pace /km", "Pace /mi"},
		TrendCols: []int{2, 3},
	}
	if *mph {
		t.TrendCols = append(t.TrendCols, len(t.Headers))
		t.Headers = append(t.Headers, "MPH")
	}
	if *kph {
		t.TrendCols = append(t.TrendCols, len(t.Headers))
		t.Headers = append(t.Headers, "KPH")
	}
	t.Headers = append(t.Headers, "Split Time", "Total Time")

	// row builds a table row; splitTime is empty for marker rows.
	row := func(label string, s Split, splitTime string, highlight bool) Row {
		km, mi := paces(s.Pace)
		cells := []string{label, fmtDist(s.Meters, unit), km, mi}
		if *mph {
			cells = append(cells, mphOf(s.Pace))
		}
		if *kph {
			cells = append(cells, kphOf(s.Pace))
		}
		cells = append(cells, splitTime, FormatClock(s.Elapsed))
		return Row{Cells: cells, Highlight: highlight, Trend: trend(s.Pace)}
	}
	next := 0 // the first marker not yet placed
	for _, s := range splits {
		var tags []string
		for ; next < len(markers) && markers[next].Meters <= s.Meters+0.5; next++ {
			m := markers[next]
			if math.Abs(s.Meters-m.Meters) < 0.5 {
				tags = append(tags, m.Short)
				continue
			}
			at, _ := SplitAt(splits, m.Meters)
			t.Rows = append(t.Rows, row(m.Label, at, "", true))
		}
		label := strconv.Itoa(s.Number)
		if len(tags) > 0 {
			label += " (" + strings.Join(tags, ", ") + ")"
		}
		t.Rows = append(t.Rows, row(label, s, FormatClock(s.Time), len(tags) > 0))
	}
	r.Table = t
	return r.write(out, *md)
}

// parseFlags parses args into fs and rejects anything left over. The flag
// package stops at the first non-flag argument, so without this check
// "-every 2 miles -mph" would quietly ignore both "miles" and "-mph".
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w (run 'rabbit -h' for help)", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q (quote values that contain spaces, e.g. -every \"2 miles\", or write 2mi)", fs.Arg(0))
	}
	return nil
}

// currentVersion returns the version set by -ldflags in release builds.
// Otherwise it falls back to the module version Go records in binaries
// built with "go install ...@v0.1.1", trimming the "v" to match releases.
func currentVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}

// equivTable lists predicted times for a runner with the given VDOT at
// the standard race distances. The race they ran (meters in total) is
// highlighted, and added as its own row if it isn't a standard distance.
func equivTable(score, meters float64, total time.Duration, unit Unit, mph, kph bool) *Table {
	t := &Table{Headers: []string{"Race", "Distance (" + unit.Name + ")", "Time", "Pace /km", "Pace /mi"}}
	if mph {
		t.Headers = append(t.Headers, "MPH")
	}
	if kph {
		t.Headers = append(t.Headers, "KPH")
	}
	row := func(name string, m float64, d time.Duration, ran bool) Row {
		p := PaceFor(d, m, unit)
		cells := []string{
			name, fmtDist(m, unit), FormatClock(d),
			FormatClock(PaceFor(d, m, Kilometer)), FormatClock(PaceFor(d, m, Mile)),
		}
		if mph {
			cells = append(cells, fmt.Sprintf("%.1f", MPH(p, unit)))
		}
		if kph {
			cells = append(cells, fmt.Sprintf("%.1f", KPH(p, unit)))
		}
		return Row{Cells: cells, Highlight: ran}
	}

	added := false
	for _, race := range equivRaces {
		ran := math.Abs(race.Meters-meters) < 1
		if !added && race.Meters > meters+1 {
			// The race they ran isn't a standard one; slot it in here.
			t.Rows = append(t.Rows, row(fmtDist(meters, unit)+" "+unit.Name, meters, total, true))
			added = true
		}
		if ran {
			// Show their actual time rather than a prediction of it.
			t.Rows = append(t.Rows, row(race.Name, meters, total, true))
			added = true
			continue
		}
		t.Rows = append(t.Rows, row(race.Name, race.Meters, RaceTime(score, race.Meters), false))
	}
	return t
}

// bothDist formats meters in km and miles, e.g. "10 km · 6.21 mi".
func bothDist(meters float64) string {
	return fmtDist(meters, Kilometer) + " km · " + fmtDist(meters, Mile) + " mi"
}

// fmtDist formats meters in unit with up to two decimals, trimming
// trailing zeros (10 rather than 10.00, 13.11 rather than 13.110).
func fmtDist(meters float64, u Unit) string {
	return strconv.FormatFloat(math.Round(meters/u.Meters*100)/100, 'f', -1, 64)
}

// Marker is an extra row in the split table, such as halfway or a fuel
// stop. Short labels the split it lands on, when it falls on a boundary.
type Marker struct {
	Label, Short string
	Meters       float64
}

// sortMarkers orders markers by distance, keeping ties in the given order.
func sortMarkers(ms []Marker) {
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].Meters < ms[j].Meters })
}

// fuelMarkers returns a marker for each fuel stop before the finish.
func fuelMarkers(f FuelEvery, splits []Split) []Marker {
	finish := splits[len(splits)-1]
	var ms []Marker
	for n := 1; ; n++ {
		var at float64
		switch {
		case f.Time > 0:
			t := time.Duration(n) * f.Time
			if t >= finish.Elapsed-time.Second {
				return ms
			}
			at = MetersAt(splits, t)
		case f.Meters > 0:
			at = float64(n) * f.Meters
			if at >= finish.Meters-0.5 {
				return ms
			}
		default:
			return ms
		}
		ms = append(ms, Marker{Label: "Fuel " + strconv.Itoa(n), Short: "fuel " + strconv.Itoa(n), Meters: at})
	}
}
