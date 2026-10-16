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

Usage:
  rabbit pace   -time <time> -dist <distance> [-mph] [-md]
  rabbit splits (-pace <pace> | -time <time>) -dist <distance>
                [-unit km|mi] [-every <distance>] [-halfway] [-mph] [-md]
                [-neg <percent> | -pos <percent>]
  rabbit version

Commands:
  pace     Work out pace (min/km and min/mi) from a finish time and distance.
  splits   Print a split table for a distance run at a given pace, or the
           pace needed for a finish time. -halfway adds a halfway row.
           -neg 5 starts 5% slower and finishes 5% faster (negative split);
           -pos 5 does the reverse. The finish time stays the same.
  version  Print the version.

  -mph adds speed in miles per hour (for treadmills) to either command.
  -md prints Markdown instead, ready to paste into Obsidian or a README.

Formats:
  time/pace  hh:mm:ss, mm:ss, or Go durations like 45m30s
  distance   10k, 5km, 13.1mi, 800m, half, marathon
             (a bare number like 10 uses km, or -unit for splits)

Examples:
  rabbit pace -time 45:30 -dist 10k
  rabbit splits -pace 4:30 -dist half
  rabbit splits -pace 8:00 -unit mi -dist 26.2
  rabbit splits -pace 4:30 -dist marathon -every 5k
  rabbit splits -pace 4:30 -dist half -neg 5
  rabbit splits -time 3:00:00 -dist marathon -every 5k -halfway -neg 2
  rabbit splits -pace 7:00 -unit mi -dist 10k -md
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
	case "pace":
		return runPace(args[1:], out)
	case "splits":
		return runSplits(args[1:], out)
	case "version", "-version", "--version":
		fmt.Fprintln(out, "rabbit", currentVersion())
		return nil
	case "help", "-h", "-help", "--help":
		fmt.Fprint(out, usage)
		return nil
	}
	return fmt.Errorf("unknown command %q (run 'rabbit help')", args[0])
}

func runPace(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pace", flag.ContinueOnError)
	timeStr := fs.String("time", "", "finish time, e.g. 45:30 or 1:45:00")
	distStr := fs.String("dist", "", "distance, e.g. 10k, 13.1mi, marathon")
	mph := fs.Bool("mph", false, "also show speed in miles per hour")
	md := fs.Bool("md", false, "print Markdown (e.g. for Obsidian) instead of styled text")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *timeStr == "" || *distStr == "" {
		return errors.New("pace needs both -time and -dist")
	}

	total, err := ParseClock(*timeStr)
	if err != nil {
		return err
	}
	meters, err := ParseDistance(*distStr, Kilometer)
	if err != nil {
		return err
	}

	r := Report{Summary: []Field{
		{"Distance", fmtDist(meters, Kilometer) + " km · " + fmtDist(meters, Mile) + " mi"},
		{"Time", FormatClock(total)},
		{"Pace", FormatClock(PaceFor(total, meters, Kilometer)) + " /km · " + FormatClock(PaceFor(total, meters, Mile)) + " /mi"},
	}}
	if *mph {
		r.Summary = append(r.Summary, Field{"Speed", fmt.Sprintf("%.1f mph", MPH(PaceFor(total, meters, Mile), Mile))})
	}
	return r.write(out, *md)
}

func runSplits(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("splits", flag.ContinueOnError)
	paceStr := fs.String("pace", "", "pace per unit, e.g. 4:30")
	timeStr := fs.String("time", "", "finish time instead of -pace, e.g. 3:00:00")
	distStr := fs.String("dist", "", "distance, e.g. 10k, 13.1mi, marathon")
	unitStr := fs.String("unit", "km", "unit of the pace and splits: km or mi")
	everyStr := fs.String("every", "", "split length, e.g. 5k or 1mi (default: one -unit)")
	halfway := fs.Bool("halfway", false, "add a row at the halfway point")
	mph := fs.Bool("mph", false, "add a speed column in miles per hour")
	neg := fs.Float64("neg", 0, "negative split: start this percent slower, finish this percent faster")
	pos := fs.Float64("pos", 0, "positive split: start this percent faster, finish this percent slower")
	md := fs.Bool("md", false, "print Markdown (e.g. for Obsidian) instead of a styled table")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if (*paceStr == "") == (*timeStr == "") || *distStr == "" {
		return errors.New("splits needs -dist and one of -pace or -time")
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
	meters, err := ParseDistance(*distStr, unit)
	if err != nil {
		return err
	}
	var pace time.Duration
	if *timeStr != "" {
		total, err := ParseClock(*timeStr)
		if err != nil {
			return err
		}
		pace = PaceFor(total, meters, unit)
	} else if pace, err = ParseClock(*paceStr); err != nil {
		return err
	}
	interval := unit.Meters
	if *everyStr != "" {
		if interval, err = ParseDistance(*everyStr, unit); err != nil {
			return err
		}
	}

	splits := Splits(pace, unit, interval, meters, skew)
	first, last := splits[0], splits[len(splits)-1]

	// paces formats a pace per unit in both km and mi.
	paces := func(p time.Duration) (string, string) {
		return FormatClock(PaceFor(p, unit.Meters, Kilometer)), FormatClock(PaceFor(p, unit.Meters, Mile))
	}
	speed := func(p time.Duration) string {
		return fmt.Sprintf("%.1f", MPH(p, unit))
	}
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
		{"Distance", fmtDist(meters, Kilometer) + " km · " + fmtDist(meters, Mile) + " mi"},
		{"Target pace", km + " /km · " + mi + " /mi"},
	}}
	if *mph {
		r.Summary = append(r.Summary, Field{"Speed", speed(pace) + " mph"})
	}
	if skew != 0 {
		label, pct := "Negative split", *neg
		if skew < 0 {
			label, pct = "Positive split", *pos
		}
		firstKm, firstMi := paces(first.Pace)
		lastKm, lastMi := paces(last.Pace)
		r.Summary = append(r.Summary, Field{
			fmt.Sprintf("%s %g%%", label, pct),
			firstKm + " → " + lastKm + " /km · " + firstMi + " → " + lastMi + " /mi",
		})
	}
	r.Summary = append(r.Summary, Field{"Finish", FormatClock(last.Elapsed)})

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
		t.Headers = append(t.Headers, "MPH")
		t.TrendCols = append(t.TrendCols, 4)
	}
	t.Headers = append(t.Headers, "Split Time", "Total Time")

	// row builds a table row; splitTime is empty for the halfway row.
	row := func(label string, s Split, splitTime string, isHalf bool) Row {
		km, mi := paces(s.Pace)
		cells := []string{label, fmtDist(s.Meters, unit), km, mi}
		if *mph {
			cells = append(cells, speed(s.Pace))
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
		return err
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

// fmtDist formats meters in unit with up to two decimals, trimming
// trailing zeros (10 rather than 10.00, 13.11 rather than 13.110).
func fmtDist(meters float64, u Unit) string {
	return strconv.FormatFloat(math.Round(meters/u.Meters*100)/100, 'f', -1, 64)
}
