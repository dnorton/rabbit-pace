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
  rabbit version

Flags:
  -time      finish time
  -pace      pace per -unit
  -dist      distance
  -unit      km or mi (default km): the unit of -pace, of a bare -dist or
             -every number, and of the split table
  -every     split length, e.g. 5k or 1mi (default: one -unit)
  -halfway   add a row at the halfway point
  -neg 5     negative split: start 5% slower, finish 5% faster
  -pos 5     positive split: start 5% faster, finish 5% slower
             (the finish time stays the same either way)
  -mph       add speed in miles per hour, for treadmills
  -kph       add speed in kilometers per hour, for treadmills
  -md        print Markdown, ready to paste into Obsidian or a README
  -only      print just the summary, without the split table

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
	if *only && (*everyStr != "" || *halfway) {
		return errors.New("-every and -halfway change the split table, which -only leaves out")
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
	if *only {
		return r.write(out, *md)
	}

	// Work out where the halfway row goes. If halfway falls on a split
	// boundary, that split is labelled instead of adding a row.
	halfIdx, halfOnSplit := -1, false
	var half Split
	if *halfway {
		half, halfIdx = SplitAt(splits, meters/2)
		halfOnSplit = math.Abs(splits[halfIdx].Meters-meters/2) < 0.5
	}

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

	// row builds a table row; splitTime is empty for the halfway row.
	row := func(label string, s Split, splitTime string, isHalf bool) Row {
		km, mi := paces(s.Pace)
		cells := []string{label, fmtDist(s.Meters, unit), km, mi}
		if *mph {
			cells = append(cells, mphOf(s.Pace))
		}
		if *kph {
			cells = append(cells, kphOf(s.Pace))
		}
		cells = append(cells, splitTime, FormatClock(s.Elapsed))
		return Row{Cells: cells, Half: isHalf, Trend: trend(s.Pace)}
	}
	for i, s := range splits {
		label := strconv.Itoa(s.Number)
		if i == halfIdx {
			if halfOnSplit {
				label += " (half)"
			} else {
				t.Rows = append(t.Rows, row("Half", half, "", true))
			}
		}
		t.Rows = append(t.Rows, row(label, s, FormatClock(s.Time), i == halfIdx && halfOnSplit))
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

// bothDist formats meters in km and miles, e.g. "10 km · 6.21 mi".
func bothDist(meters float64) string {
	return fmtDist(meters, Kilometer) + " km · " + fmtDist(meters, Mile) + " mi"
}

// fmtDist formats meters in unit with up to two decimals, trimming
// trailing zeros (10 rather than 10.00, 13.11 rather than 13.110).
func fmtDist(meters float64, u Unit) string {
	return strconv.FormatFloat(math.Round(meters/u.Meters*100)/100, 'f', -1, 64)
}
