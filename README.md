# rabbit

A command-line running pace calculator.
Named after the "rabbits": the pacers who set the tempo in a race.

```
$ rabbit splits -time 3:00:00 -dist marathon -every 5k -halfway -neg 2
Distance           42.2 km · 26.22 mi
Target pace        4:16 /km · 6:52 /mi
Negative split 2%  4:21 → 4:11 /km · 7:00 → 6:43 /mi
Finish             3:00:00

╭───────┬───────────────┬──────────┬──────────┬────────────┬────────────╮
│ Split │ Distance (km) │ Pace /km │ Pace /mi │ Split Time │ Total Time │
├───────┼───────────────┼──────────┼──────────┼────────────┼────────────┤
│     1 │             5 │     4:21 │     7:00 │      21:44 │      21:44 │
│     2 │            10 │     4:19 │     6:58 │      21:37 │      43:21 │
│     3 │            15 │     4:18 │     6:55 │      21:31 │    1:04:52 │
│     4 │            20 │     4:17 │     6:53 │      21:24 │    1:26:16 │
│  Half │          21.1 │     4:16 │     6:51 │            │    1:30:57 │
│     5 │            25 │     4:16 │     6:51 │      21:18 │    1:47:34 │
│     6 │            30 │     4:14 │     6:49 │      21:12 │    2:08:46 │
│     7 │            35 │     4:13 │     6:47 │      21:05 │    2:29:51 │
│     8 │            40 │     4:12 │     6:45 │      20:59 │    2:50:50 │
│     9 │          42.2 │     4:11 │     6:43 │       9:10 │    3:00:00 │
╰───────┴───────────────┴──────────┴──────────┴────────────┴────────────╯
```

In a terminal the table is coloured: splits faster than the target pace are
green, slower ones yellow, and the halfway row is highlighted. Colours are
turned off automatically when output is piped or `NO_COLOR` is set.

## Install

### Linux and macOS

```bash
curl -fsSL https://raw.githubusercontent.com/dnorton/rabbit-pace/main/install.sh | sh
```

This downloads the latest release for your OS and CPU, checks it against the
release's `checksums.txt`, and installs `rabbit` to `~/.local/bin`. Run it
again to upgrade; delete `~/.local/bin/rabbit` to uninstall.

Prefer to read the script before running it?

```bash
curl -fsSLO https://raw.githubusercontent.com/dnorton/rabbit-pace/main/install.sh
less install.sh
sh install.sh
```

Options are set with environment variables:

```bash
# Install a specific version
curl -fsSL .../install.sh | RABBIT_VERSION=v0.1.0 sh

# Install system-wide
curl -fsSL .../install.sh | sudo RABBIT_INSTALL_DIR=/usr/local/bin sh
```

### Packages and other options

Packages and binaries for each release (including Windows `.zip` files) are
on the [releases page](https://github.com/dnorton/rabbit-pace/releases).

```bash
# Debian / Ubuntu
sudo apt install ./rabbit-pace_<version>_linux_amd64.deb

# Fedora / RHEL
sudo dnf install ./rabbit-pace_<version>_linux_amd64.rpm

# Alpine
sudo apk add --allow-untrusted ./rabbit-pace_<version>_linux_amd64.apk

# Arch
sudo pacman -U ./rabbit-pace_<version>_linux_amd64.pkg.tar.zst

# Anywhere with Go installed
go install github.com/dnorton/rabbit-pace/cmd/rabbit@latest

# Docker
docker run --rm ghcr.io/dnorton/rabbit-pace splits -pace 4:30 -dist 10k
```

The Linux package is called `rabbit-pace` (the command is still `rabbit`)
because Debian and Ubuntu already have an unrelated `rabbit` package.

## Usage

### Pace, finish time or distance

Give `rabbit pace` any two of `-time`, `-pace` and `-dist`, and it works out
the third, highlighted in the terminal. `-unit km|mi` sets the unit of
`-pace` (km by default).

```
# Pace from time + distance
$ rabbit pace -time 45:30 -dist 10k
Distance  10 km · 6.21 mi
Time      45:30
Pace      4:33 /km · 7:19 /mi

# Finish time from pace + distance
$ rabbit pace -pace 7:00 -unit mi -dist half -mph
Distance  21.1 km · 13.11 mi
Time      1:31:46
Pace      4:21 /km · 7:00 /mi
Speed     8.6 mph

# Distance from pace + time
$ rabbit pace -pace 5:00 -time 30:00
Distance  6 km · 3.73 mi
Time      30:00
Pace      5:00 /km · 8:03 /mi
```

### Split tables

```bash
# Split table for a distance at a given pace (-unit defaults to km)
rabbit splits -pace 4:30 -dist half
rabbit splits -pace 8:00 -unit mi -dist 26.2

# Custom split length with -every (defaults to one -unit)
rabbit splits -pace 4:30 -dist marathon -every 5k

# Negative split: start 5% slower, finish 5% faster, same finish time
rabbit splits -pace 4:30 -dist half -neg 5

# Positive split: start 3% faster, finish 3% slower
rabbit splits -pace 8:00 -unit mi -dist 10k -pos 3

# Use a finish time instead of a pace, and add a halfway row
rabbit splits -time 3:00:00 -dist marathon -every 5k -halfway -neg 2

# Add treadmill speed in mph (works with pace too)
rabbit splits -pace 7:00 -unit mi -dist 5k -mph
```

The split table shows pace in both min/km and min/mi.

### Markdown (Obsidian, READMEs)

Add `-md` to either command to print Markdown instead: the summary as a list
(with the calculated value in bold) and the splits as a table that renders
properly when pasted into Obsidian or any Markdown editor.

```bash
# Straight to the clipboard
rabbit splits -pace 7:00 -unit mi -dist 5k -halfway -md | wl-copy          # Wayland
rabbit splits -pace 7:00 -unit mi -dist 5k -halfway -md | xclip -sel clip  # X11
rabbit splits -pace 7:00 -unit mi -dist 5k -halfway -md | pbcopy           # macOS
```

That prints:

```markdown
- **Distance:** 5 km · 3.11 mi
- **Target pace:** 4:21 /km · 7:00 /mi
- **Finish:** 21:45

| Split | Distance (mi) | Pace /km | Pace /mi | Split Time | Total Time |
|---:|---:|---:|---:|---:|---:|
| 1 | 1 | 4:21 | 7:00 | 7:00 | 7:00 |
| **Half** | 1.55 | 4:21 | 7:00 |  | 10:52 |
| 2 | 2 | 4:21 | 7:00 | 7:00 | 14:00 |
| 3 | 3 | 4:21 | 7:00 | 7:00 | 21:00 |
| 4 | 3.11 | 4:21 | 7:00 | 0:45 | 21:45 |
```

which renders as:

- **Distance:** 5 km · 3.11 mi
- **Target pace:** 4:21 /km · 7:00 /mi
- **Finish:** 21:45

| Split | Distance (mi) | Pace /km | Pace /mi | Split Time | Total Time |
|---:|---:|---:|---:|---:|---:|
| 1 | 1 | 4:21 | 7:00 | 7:00 | 7:00 |
| **Half** | 1.55 | 4:21 | 7:00 |  | 10:52 |
| 2 | 2 | 4:21 | 7:00 | 7:00 | 14:00 |
| 3 | 3 | 4:21 | 7:00 | 7:00 | 21:00 |
| 4 | 3.11 | 4:21 | 7:00 | 0:45 | 21:45 |

### Input formats

Times and paces accept `hh:mm:ss`, `mm:ss`, or Go durations like `45m30s`.
Distances accept `10k`, `5km`, `13.1mi`, `800m`, `half`, `marathon`, or a bare
number in `-unit` (km by default). Quote distances that contain a space
(`-every "2 miles"`) or leave the space out (`2mi`).

## Development

```bash
go run ./cmd/rabbit splits -pace 4:30 -dist 10k   # run from source
go test ./...                                     # run the tests
go build ./cmd/rabbit                             # creates ./rabbit
docker build -t rabbit .                          # build the Docker image
```

## Releasing

Push a version tag and GitHub Actions does the rest:

```bash
git tag v0.1.0
git push origin v0.1.0
```

[`.github/workflows/release.yml`](.github/workflows/release.yml) runs
[GoReleaser](https://goreleaser.com) (config in
[`.goreleaser.yaml`](.goreleaser.yaml)) to build binaries and Linux packages
and attach them to a GitHub release, and pushes a multi-arch Docker image to
`ghcr.io/dnorton/rabbit-pace`.

To try a release locally without publishing anything:

```bash
goreleaser release --snapshot --clean   # output goes to ./dist
```

## License

[MIT](LICENSE)
