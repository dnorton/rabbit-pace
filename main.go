// Command pacecalc is a running pace calculator.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"text/tabwriter"
	"time"
)

const usage = `pacecalc - running pace calculator

Usage:
  pacecalc pace   -time <time> -dist <distance>
  pacecalc splits (-pace <pace> | -time <time>) -dist <distance>
                  [-unit km|mi] [-every <distance>] [-halfway]
                  [-neg <percent> | -pos <percent>]

Commands:
  pace     Work out pace (min/km and min/mi) from a finish time and distance.
  splits   Print a split table for a distance run at a given pace, or the
           pace needed for a finish time. -halfway adds a halfway row.
           -neg 5 starts 5% slower and finishes 5% faster (negative split);
           -pos 5 does the reverse. The finish time stays the same.

Formats:
  time/pace  hh:mm:ss, mm:ss, or Go durations like 45m30s
  distance   10k, 5km, 13.1mi, 800m, half, marathon
             (a bare number like 10 uses km, or -unit for splits)

Examples:
  pacecalc pace -time 45:30 -dist 10k
  pacecalc splits -pace 4:30 -dist half
  pacecalc splits -pace 8:00 -unit mi -dist 26.2
  pacecalc splits -pace 4:30 -dist marathon -every 5k
  pacecalc splits -pace 4:30 -dist half -neg 5
  pacecalc splits -time 3:00:00 -dist marathon -every 5k -halfway -neg 2
`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "pacecalc:", err)
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
	case "help", "-h", "-help", "--help":
		fmt.Fprint(out, usage)
		return nil
	}
	return fmt.Errorf("unknown command %q (run 'pacecalc help')", args[0])
}

func runPace(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pace", flag.ContinueOnError)
	timeStr := fs.String("time", "", "finish time, e.g. 45:30 or 1:45:00")
	distStr := fs.String("dist", "", "distance, e.g. 10k, 13.1mi, marathon")
	if err := fs.Parse(args); err != nil {
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

	fmt.Fprintf(out, "Distance: %s km / %s mi\n", fmtDist(meters, Kilometer), fmtDist(meters, Mile))
	fmt.Fprintf(out, "Time:     %s\n", FormatClock(total))
	fmt.Fprintf(out, "Pace:     %s /km\n", FormatClock(PaceFor(total, meters, Kilometer)))
	fmt.Fprintf(out, "          %s /mi\n", FormatClock(PaceFor(total, meters, Mile)))
	return nil
}

func runSplits(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("splits", flag.ContinueOnError)
	paceStr := fs.String("pace", "", "pace per unit, e.g. 4:30")
	timeStr := fs.String("time", "", "finish time instead of -pace, e.g. 3:00:00")
	distStr := fs.String("dist", "", "distance, e.g. 10k, 13.1mi, marathon")
	unitStr := fs.String("unit", "km", "unit of the pace and splits: km or mi")
	everyStr := fs.String("every", "", "split length, e.g. 5k or 1mi (default: one -unit)")
	halfway := fs.Bool("halfway", false, "add a row at the halfway point")
	neg := fs.Float64("neg", 0, "negative split: start this percent slower, finish this percent faster")
	pos := fs.Float64("pos", 0, "positive split: start this percent faster, finish this percent slower")
	if err := fs.Parse(args); err != nil {
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

	// paces formats a pace per unit as "km pace / mi pace".
	paces := func(p time.Duration) (string, string) {
		return FormatClock(PaceFor(p, unit.Meters, Kilometer)), FormatClock(PaceFor(p, unit.Meters, Mile))
	}

	km, mi := paces(pace)
	fmt.Fprintf(out, "Target pace: %s /km, %s /mi\n", km, mi)
	if skew != 0 {
		kind, pct := "Negative", *neg
		if skew < 0 {
			kind, pct = "Positive", *pos
		}
		firstKm, firstMi := paces(first.Pace)
		lastKm, lastMi := paces(last.Pace)
		fmt.Fprintf(out, "%s split %g%%: %s -> %s /km, %s -> %s /mi\n", kind, pct, firstKm, lastKm, firstMi, lastMi)
	}
	fmt.Fprintf(out, "Finish:      %s\n\n", FormatClock(last.Elapsed))

	// Work out where the halfway row goes. If halfway falls on a split
	// boundary, that split is labelled instead of adding a row.
	halfIdx, halfOnSplit := -1, false
	var half Split
	if *halfway {
		half, halfIdx = SplitAt(splits, meters/2)
		halfOnSplit = math.Abs(splits[halfIdx].Meters-meters/2) < 0.5
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintf(tw, "Split\tDistance (%s)\tPace /km\tPace /mi\tSplit Time\tTotal Time\t\n", unit.Name)
	for i, s := range splits {
		label := strconv.Itoa(s.Number)
		if i == halfIdx {
			if halfOnSplit {
				label += " (half)"
			} else {
				km, mi := paces(half.Pace)
				fmt.Fprintf(tw, "Half\t%s\t%s\t%s\t\t%s\t\n",
					fmtDist(half.Meters, unit), km, mi, FormatClock(half.Elapsed))
			}
		}
		km, mi := paces(s.Pace)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t\n",
			label, fmtDist(s.Meters, unit), km, mi, FormatClock(s.Time), FormatClock(s.Elapsed))
	}
	return tw.Flush()
}

// fmtDist formats meters in unit with up to two decimals, trimming
// trailing zeros (10 rather than 10.00, 13.11 rather than 13.110).
func fmtDist(meters float64, u Unit) string {
	return strconv.FormatFloat(math.Round(meters/u.Meters*100)/100, 'f', -1, 64)
}
