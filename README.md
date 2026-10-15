# pacecalc

A command-line running pace calculator, written as a project for learning Go.

## Usage

```bash
# Pace from a finish time and distance (shows min/km and min/mi)
pacecalc pace -time 45:30 -dist 10k

# Split table for a distance at a given pace (-unit defaults to km)
pacecalc splits -pace 4:30 -dist half
pacecalc splits -pace 8:00 -unit mi -dist 26.2

# Custom split length with -every (defaults to one -unit)
pacecalc splits -pace 4:30 -dist marathon -every 5k

# Negative split: start 5% slower, finish 5% faster, same finish time
pacecalc splits -pace 4:30 -dist half -neg 5

# Positive split: start 3% faster, finish 3% slower
pacecalc splits -pace 8:00 -unit mi -dist 10k -pos 3

# Use a finish time instead of a pace, and add a halfway row
pacecalc splits -time 3:00:00 -dist marathon -every 5k -halfway -neg 2
```

The split table shows pace in both min/km and min/mi.

Times and paces accept `hh:mm:ss`, `mm:ss`, or Go durations like `45m30s`.
Distances accept `10k`, `5km`, `13.1mi`, `800m`, `half`, `marathon`, or a bare
number (km for `pace`, the `-unit` for `splits`).

## Test

```bash
go test ./...
```

## Run

```bash
go run .
```

## Build

```bash
go build      # creates ./pacecalc
./pacecalc
```
