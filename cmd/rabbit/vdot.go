package main

import (
	"fmt"
	"math"
	"time"
)

// VDOT is Jack Daniels' running fitness score: the VO2max (ml/kg/min) a
// race performance implies, from the Daniels/Gilbert equations. It folds
// in running economy, so it's a performance measure rather than a lab
// VO2max.

// The equations were fitted to races from roughly 1500 m to the marathon,
// and drift outside that range.
const (
	vdotMinMeters = 1500
	vdotMaxMeters = 42195
)

// vo2Cost is the oxygen cost (ml/kg/min) of running at v meters/minute.
func vo2Cost(v float64) float64 {
	return -4.60 + 0.182258*v + 0.000104*v*v
}

// vo2Fraction is the fraction of VO2max a runner can sustain for a race
// lasting t minutes.
func vo2Fraction(t float64) float64 {
	return 0.8 + 0.1894393*math.Exp(-0.012778*t) + 0.2989558*math.Exp(-0.1932605*t)
}

// checkVDOTRange reports an error if meters is outside the distances the
// VDOT equations are good for.
func checkVDOTRange(meters float64) error {
	if meters < vdotMinMeters-0.5 || meters > vdotMaxMeters+0.5 {
		return fmt.Errorf("VDOT needs a race between 1500 m and the marathon, not %s", bothDist(meters))
	}
	return nil
}

// VDOT returns the VDOT for running meters in total time.
func VDOT(meters float64, total time.Duration) float64 {
	t := total.Minutes()
	return vo2Cost(meters/t) / vo2Fraction(t)
}

// RaceTime returns the time a runner with the given VDOT is predicted to
// run meters in: the inverse of VDOT. VDOT falls as time rises, so it's
// found by bisection.
func RaceTime(vdot, meters float64) time.Duration {
	lo, hi := 1.0, 1000.0 // minutes
	for range 100 {
		mid := (lo + hi) / 2
		if VDOT(meters, time.Duration(mid*float64(time.Minute))) > vdot {
			lo = mid // too fast for this VDOT
		} else {
			hi = mid
		}
	}
	return time.Duration((lo + hi) / 2 * float64(time.Minute))
}

// Race is a standard race distance for the equivalents table.
type Race struct {
	Name   string
	Meters float64
}

// equivRaces are the races shown by -equiv.
var equivRaces = []Race{
	{"1500m", 1500},
	{"Mile", 1609.344},
	{"3K", 3000},
	{"5K", 5000},
	{"10K", 10000},
	{"15K", 15000},
	{"10 Mile", 16093.44},
	{"Half", 21097.5},
	{"Marathon", 42195},
}
